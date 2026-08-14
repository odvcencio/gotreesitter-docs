//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"syscall/js"
	"time"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter-docs/internal/playgroundengine"
	"github.com/odvcencio/gotreesitter/grammars"
	enginewasm "m31labs.dev/gosx/engine/wasm"
)

const expectedGrammarCount = 206

type browserListener struct {
	target js.Value
	event  string
	fn     js.Func
}

type engineProps struct {
	GrammarIndexURL string `json:"grammarIndexURL"`
}

type grammarAsset struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Bytes int    `json:"bytes"`
}

type playgroundHandle struct {
	mount     js.Value
	listeners []browserListener

	stateMu  sync.Mutex
	timer    *time.Timer
	disposed bool
	ready    bool
	runSeq   uint64
	assets   map[string]grammarAsset

	// seeded records the sample last written into the editors. Switching
	// language only replaces the editors when they still hold exactly this
	// text, so a visitor's own code is never discarded by a grammar change.
	seeded languageSample

	languageMu sync.Mutex
	languages  map[string]*gts.Language
}

func main() {
	if err := enginewasm.Register("GotreesitterPlayground", mountPlayground); err != nil {
		panic(err)
	}
	select {}
}

func mountPlayground(ctx enginewasm.Context) (enginewasm.Handle, error) {
	mount := ctx.Mount()
	if !mount.Truthy() {
		return nil, fmt.Errorf("playground requires a DOM mount")
	}
	var props engineProps
	if err := ctx.DecodeProps(&props); err != nil {
		return nil, err
	}
	if props.GrammarIndexURL == "" {
		return nil, fmt.Errorf("playground grammar index URL is missing")
	}
	h := &playgroundHandle{
		mount:     mount,
		assets:    map[string]grammarAsset{},
		languages: map[string]*gts.Language{},
	}
	for _, binding := range []struct {
		selector  string
		event     string
		immediate bool
	}{
		{"#pg-source", "input", false},
		{"#pg-query", "input", false},
		// #pg-language is deliberately absent: its handler below swaps in the
		// new language's starter sample first, then re-parses. Binding it here
		// too would re-parse the old source before the swap landed.
		{"#pg-anonymous", "change", true},
		{"#pg-parse", "click", true},
	} {
		target := h.find(binding.selector)
		if !target.Truthy() {
			return nil, fmt.Errorf("playground element %s is missing", binding.selector)
		}
		immediate := binding.immediate
		fn := js.FuncOf(func(js.Value, []js.Value) any {
			if immediate {
				h.run()
			} else {
				h.schedule()
			}
			return nil
		})
		target.Call("addEventListener", binding.event, fn)
		h.listeners = append(h.listeners, browserListener{target: target, event: binding.event, fn: fn})
	}
	// #pg-source gets two extra, non-debounced listeners on top of the
	// schedule()-driven "input" binding registered above:
	//   - a plain-text mirror into #pg-hl on every keystroke, so the
	//     transparent-text textarea (see .pg-src in playground.css) never
	//     shows an invisible character during the 180ms debounce window
	//     before the real, classified highlight (h.renderHighlight) lands.
	//   - a scroll listener keeping #pg-hl's scroll position locked to
	//     #pg-source's, since the highlight overlay sits behind the
	//     (invisible-text) textarea the visitor actually scrolls.
	source := h.find("#pg-source")
	if !source.Truthy() {
		return nil, fmt.Errorf("playground element #pg-source is missing")
	}
	mirrorFn := js.FuncOf(func(js.Value, []js.Value) any {
		h.mirrorPlainSource()
		return nil
	})
	source.Call("addEventListener", "input", mirrorFn)
	h.listeners = append(h.listeners, browserListener{target: source, event: "input", fn: mirrorFn})

	scrollFn := js.FuncOf(func(js.Value, []js.Value) any {
		h.syncHighlightScroll()
		return nil
	})
	source.Call("addEventListener", "scroll", scrollFn)
	h.listeners = append(h.listeners, browserListener{target: source, event: "scroll", fn: scrollFn})

	// Switching grammar swaps in that language's starter sample, so the picker
	// never leaves Go source sitting under a Rust parser. The swap is skipped
	// the moment the visitor has typed anything of their own: see swapSample.
	languageEl := h.find("#pg-language")
	if !languageEl.Truthy() {
		return nil, fmt.Errorf("playground element #pg-language is missing")
	}
	// Seed from whatever the server rendered, so the first grammar change can
	// recognise the untouched initial sample.
	h.seeded = languageSample{
		Source: source.Get("value").String(),
		Query:  h.find("#pg-query").Get("value").String(),
	}
	languageFn := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		h.swapSample(languageEl.Get("value").String())
		h.run()
		return nil
	})
	languageEl.Call("addEventListener", "change", languageFn)
	h.listeners = append(h.listeners, browserListener{target: languageEl, event: "change", fn: languageFn})

	// Selecting a syntax tree row selects the source it covers. The listener is
	// delegated to #pg-tree because render() rebuilds every row on each parse,
	// so per-row listeners would leak and would have to be rebound constantly.
	// Rows carry data-start/data-end in UTF-16 units (see render).
	tree := h.find("#pg-tree")
	if !tree.Truthy() {
		return nil, fmt.Errorf("playground element #pg-tree is missing")
	}
	selectFn := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		// Keyboard activation follows the treeitem role: Enter or Space only,
		// so arrow-key navigation between rows does not hijack the caret.
		if event.Get("type").String() == "keydown" {
			switch event.Get("key").String() {
			case "Enter", " ":
			default:
				return nil
			}
			event.Call("preventDefault")
		}
		target := event.Get("target")
		if !target.Truthy() || target.Get("closest").IsUndefined() {
			return nil
		}
		row := target.Call("closest", "[data-start]")
		if !row.Truthy() {
			return nil
		}
		start, startErr := strconv.Atoi(row.Call("getAttribute", "data-start").String())
		end, endErr := strconv.Atoi(row.Call("getAttribute", "data-end").String())
		if startErr != nil || endErr != nil {
			return nil
		}
		h.selectSourceRange(start, end)
		return nil
	})
	for _, event := range []string{"click", "keydown"} {
		tree.Call("addEventListener", event, selectFn)
		h.listeners = append(h.listeners, browserListener{target: tree, event: event, fn: selectFn})
	}

	h.mount.Get("dataset").Set("privacyBoundary", "browser-only")
	go h.bootstrap(props.GrammarIndexURL)
	return h, nil
}

func (h *playgroundHandle) Dispose() {
	h.stateMu.Lock()
	h.disposed = true
	h.runSeq++
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	h.stateMu.Unlock()
	for _, listener := range h.listeners {
		listener.target.Call("removeEventListener", listener.event, listener.fn)
		listener.fn.Release()
	}
	h.listeners = nil
}

func (h *playgroundHandle) bootstrap(indexURL string) {
	var assets []grammarAsset
	if err := fetchJSON(indexURL, &assets); err != nil {
		h.fail("Could not load the grammar index: " + err.Error())
		return
	}
	if len(assets) != expectedGrammarCount {
		h.fail(fmt.Sprintf("Grammar index contains %d languages; expected %d.", len(assets), expectedGrammarCount))
		return
	}
	h.stateMu.Lock()
	if h.disposed {
		h.stateMu.Unlock()
		return
	}
	for _, asset := range assets {
		h.assets[asset.Name] = asset
	}
	h.ready = true
	h.stateMu.Unlock()

	selectBox := h.find("#pg-language")
	selectBox.Set("textContent", "")
	for _, asset := range assets {
		option := element("option")
		option.Set("value", asset.Name)
		option.Set("textContent", fmt.Sprintf("%s · %s", asset.Name, formatBytes(asset.Bytes)))
		if asset.Name == "go" {
			option.Set("selected", true)
		}
		selectBox.Call("appendChild", option)
	}
	selectBox.Set("disabled", false)
	h.mount.Get("dataset").Set("grammarCount", fmt.Sprint(len(assets)))
	if seq, ok := h.beginRun(); ok {
		h.runAsync(seq)
	}
}

func (h *playgroundHandle) schedule() {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	if h.disposed || !h.ready {
		return
	}
	if h.timer != nil {
		h.timer.Stop()
	}
	h.timer = time.AfterFunc(180*time.Millisecond, h.run)
}

func (h *playgroundHandle) run() {
	seq, ok := h.beginRun()
	if !ok {
		return
	}
	go h.runAsync(seq)
}

func (h *playgroundHandle) beginRun() (uint64, bool) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	if h.disposed || !h.ready {
		return 0, false
	}
	h.timer = nil
	h.runSeq++
	return h.runSeq, true
}

func (h *playgroundHandle) runAsync(seq uint64) {
	defer func() {
		if recovered := recover(); recovered != nil && h.current(seq) {
			h.fail(fmt.Sprintf("Browser parser failed: %v", recovered))
		}
	}()
	source := h.find("#pg-source").Get("value").String()
	query := h.find("#pg-query").Get("value").String()
	languageName := h.find("#pg-language").Get("value").String()
	includeAnonymous := h.find("#pg-anonymous").Get("checked").Bool()
	if !h.current(seq) {
		return
	}
	h.text("#pg-status", "Loading "+languageName+" grammar locally…")
	language, err := h.loadLanguage(languageName)
	if err != nil {
		if h.current(seq) {
			h.fail("Could not load " + languageName + ": " + err.Error())
		}
		return
	}
	if !h.current(seq) {
		return
	}
	h.text("#pg-status", "Parsing locally…")
	started := time.Now()
	result := playgroundengine.ParseLanguage(source, query, languageName, language, includeAnonymous)
	highlightSpans, _ := playgroundengine.ComputeHighlight(source, languageName, language)
	if h.current(seq) {
		h.render(languageName, source, result, highlightSpans, time.Since(started))
	}
}

func (h *playgroundHandle) loadLanguage(name string) (*gts.Language, error) {
	h.languageMu.Lock()
	defer h.languageMu.Unlock()
	if language := h.languages[name]; language != nil {
		return language, nil
	}
	h.stateMu.Lock()
	asset, ok := h.assets[name]
	disposed := h.disposed
	h.stateMu.Unlock()
	if disposed {
		return nil, fmt.Errorf("engine disposed")
	}
	if !ok || asset.URL == "" {
		return nil, fmt.Errorf("grammar asset is not indexed")
	}
	blob, err := fetchBytes(asset.URL)
	if err != nil {
		return nil, err
	}
	if len(blob) != asset.Bytes {
		return nil, fmt.Errorf("grammar blob has %d bytes; expected %d", len(blob), asset.Bytes)
	}
	language, err := loadFetchedLanguage(name, blob)
	if err != nil {
		return nil, err
	}
	h.languages[name] = language
	return language, nil
}

type languageBoundExternalScanner interface {
	ExternalScannerForLanguage(*gts.Language) gts.ExternalScanner
}

// loadFetchedLanguage binds runtime support without asking gotreesitter to
// reopen the same built-in blob from a filesystem. The fetched assets are the
// exact pinned-module package blobs, so unbound scanners already use matching symbol
// IDs; bound scanners derive their IDs directly from the decoded language.
func loadFetchedLanguage(name string, blob []byte) (*gts.Language, error) {
	language, err := gts.LoadLanguage(blob)
	if err != nil {
		return nil, err
	}
	language.Name = name
	if scanner := grammars.LookupExternalScanner(name); scanner != nil {
		if bound, ok := scanner.(languageBoundExternalScanner); ok {
			language.ExternalScanner = bound.ExternalScannerForLanguage(language)
		} else {
			language.ExternalScanner = scanner
		}
	}
	if states := grammars.LookupExternalLexStates(name); len(states) > 0 {
		language.ExternalLexStates = states
	}
	return language, nil
}

func (h *playgroundHandle) current(seq uint64) bool {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	return !h.disposed && h.runSeq == seq
}

func (h *playgroundHandle) fail(message string) {
	h.text("#pg-status", message)
	errors := h.find("#pg-errors")
	errors.Set("textContent", "")
	box := element("div")
	box.Set("className", "pg-qerr mono")
	box.Set("role", "alert")
	box.Set("textContent", message)
	errors.Call("appendChild", box)
}

func (h *playgroundHandle) render(language, source string, result playgroundengine.Result, highlightSpans []playgroundengine.HighlightSpan, elapsed time.Duration) {
	h.text("#pg-language-label", language)
	h.text("#pg-node-count", fmt.Sprintf("%d nodes", result.NodeCount))
	h.text("#pg-capture-count", fmt.Sprintf("%d captures", len(result.Captures)))
	h.renderHighlight(source, highlightSpans)

	tree := h.find("#pg-tree")
	tree.Set("textContent", "")
	for _, row := range result.TreeRows {
		line := element("div")
		line.Set("className", row.Class)
		line.Set("role", "treeitem")
		line.Call("setAttribute", "aria-level", fmt.Sprint(row.Level))
		// Stamp the row's source span so a click can select the exact text the
		// node covers. These are UTF-16 code-unit indices, not the byte offsets
		// gotreesitter reports: textarea.setSelectionRange counts UTF-16 units,
		// so any non-ASCII source earlier in the buffer would otherwise shift
		// the selection. selectSourceRange reads these back verbatim.
		line.Call("setAttribute", "data-start", fmt.Sprint(utf16Index(source, row.StartByte)))
		line.Call("setAttribute", "data-end", fmt.Sprint(utf16Index(source, row.EndByte)))
		line.Call("setAttribute", "tabindex", "0")
		line.Call("appendChild", textNode(row.Depth))
		if row.Field != "" {
			line.Call("appendChild", styledText("span", "tfield", row.Field+": "))
		}
		line.Call("appendChild", styledText("span", "ttype", row.Type))
		line.Call("appendChild", styledText("span", "tfield", " ["+row.Range+"]"))
		if row.Missing {
			line.Call("appendChild", styledText("span", "tmissing", "MISSING"))
		}
		tree.Call("appendChild", line)
	}
	if len(result.TreeRows) == 0 {
		empty := element("p")
		empty.Set("className", "pg-tree-empty")
		empty.Set("textContent", "No tree rows to display.")
		tree.Call("appendChild", empty)
	}

	errors := h.find("#pg-errors")
	errors.Set("textContent", "")
	for _, message := range []string{result.ParseError, result.QueryError} {
		if message == "" {
			continue
		}
		box := element("div")
		box.Set("className", "pg-qerr mono")
		box.Set("role", "alert")
		box.Set("textContent", message)
		errors.Call("appendChild", box)
	}

	captures := h.find("#pg-captures")
	captures.Set("textContent", "")
	for _, capture := range result.Captures {
		card := element("div")
		card.Set("className", "pg-capture mono")
		name := element("strong")
		name.Set("textContent", "@"+capture.Name)
		meta := element("span")
		meta.Set("textContent", fmt.Sprintf("match %d · %s", capture.Match, capture.Range))
		code := element("code")
		code.Set("textContent", capture.Text)
		card.Call("append", name, meta, code)
		captures.Call("appendChild", card)
	}

	status := fmt.Sprintf("Parsed locally in %s.", elapsed.Round(100*time.Microsecond))
	if result.ParseError != "" || result.QueryError != "" || result.HasErrors {
		status = fmt.Sprintf("Parsed locally with diagnostics in %s.", elapsed.Round(100*time.Microsecond))
	}
	h.text("#pg-status", status)
}

// renderHighlight fills #pg-hl — the highlight overlay <pre> sitting behind
// the transparent-text #pg-source textarea (see .pg-hlpre/.pg-src in
// playground.css) — with source, interleaving tk-* styled spans in byte
// order. An empty highlightSpans (no highlight query for this grammar, or a
// source with nothing a mapped capture matched — see
// playgroundengine.ComputeHighlight) still renders the full plain source as
// one unstyled text node, so the overlay never falls out of sync with what
// the visitor is actually typing: "real highlighting or none," but the text
// itself is always there.
func (h *playgroundHandle) renderHighlight(source string, spans []playgroundengine.HighlightSpan) {
	container := h.find("#pg-hl")
	if !container.Truthy() {
		return
	}
	container.Set("textContent", "")
	src := []byte(source)
	cursor := 0
	for _, span := range spans {
		start, end := span.StartByte, span.EndByte
		if start < cursor {
			start = cursor
		}
		if end > len(src) {
			end = len(src)
		}
		if start > len(src) {
			start = len(src)
		}
		if end <= start {
			continue
		}
		if start > cursor {
			container.Call("appendChild", textNode(string(src[cursor:start])))
		}
		container.Call("appendChild", styledText("span", span.Class, string(src[start:end])))
		cursor = end
	}
	if cursor < len(src) {
		container.Call("appendChild", textNode(string(src[cursor:])))
	}
}

// mirrorPlainSource copies #pg-source's current value into #pg-hl verbatim,
// unstyled. It runs synchronously on every "input" event (see
// mountPlayground's dedicated #pg-source listener), ahead of the 180ms
// debounced reparse+rehighlight (h.schedule -> h.run -> h.renderHighlight),
// so a keystroke is never invisible against the transparent-text textarea
// while waiting for the debounce to fire.
func (h *playgroundHandle) mirrorPlainSource() {
	container := h.find("#pg-hl")
	if !container.Truthy() {
		return
	}
	container.Set("textContent", h.find("#pg-source").Get("value").String())
}

// syncHighlightScroll mirrors #pg-source's scroll position onto #pg-hl so
// the highlight overlay tracks the (invisible-text) textarea the visitor is
// actually scrolling.
func (h *playgroundHandle) syncHighlightScroll() {
	container := h.find("#pg-hl")
	source := h.find("#pg-source")
	if !container.Truthy() || !source.Truthy() {
		return
	}
	container.Set("scrollTop", source.Get("scrollTop"))
	container.Set("scrollLeft", source.Get("scrollLeft"))
}

func (h *playgroundHandle) find(selector string) js.Value {
	return h.mount.Call("querySelector", selector)
}

func (h *playgroundHandle) text(selector, value string) {
	target := h.find(selector)
	if target.Truthy() {
		target.Set("textContent", value)
	}
}

func fetchJSON(url string, target any) error {
	response, err := fetchResponse(url)
	if err != nil {
		return err
	}
	value, err := await(response.Call("json"))
	if err != nil {
		return err
	}
	encoded := js.Global().Get("JSON").Call("stringify", value)
	return json.Unmarshal([]byte(encoded.String()), target)
}

func fetchBytes(url string) ([]byte, error) {
	response, err := fetchResponse(url)
	if err != nil {
		return nil, err
	}
	buffer, err := await(response.Call("arrayBuffer"))
	if err != nil {
		return nil, err
	}
	array := js.Global().Get("Uint8Array").New(buffer)
	data := make([]byte, array.Get("byteLength").Int())
	js.CopyBytesToGo(data, array)
	return data, nil
}

func fetchResponse(url string) (js.Value, error) {
	response, err := await(js.Global().Call("fetch", url))
	if err != nil {
		return js.Undefined(), err
	}
	if !response.Get("ok").Bool() {
		return js.Undefined(), fmt.Errorf("GET %s returned HTTP %d", url, response.Get("status").Int())
	}
	return response, nil
}

type promiseResult struct {
	value js.Value
	err   error
}

func await(promise js.Value) (js.Value, error) {
	done := make(chan promiseResult, 1)
	var then js.Func
	var catch js.Func
	then = js.FuncOf(func(_ js.Value, args []js.Value) any {
		value := js.Undefined()
		if len(args) > 0 {
			value = args[0]
		}
		done <- promiseResult{value: value}
		return nil
	})
	catch = js.FuncOf(func(_ js.Value, args []js.Value) any {
		message := "promise rejected"
		if len(args) > 0 {
			message = args[0].Call("toString").String()
		}
		done <- promiseResult{err: fmt.Errorf("%s", message)}
		return nil
	})
	promise.Call("then", then).Call("catch", catch)
	result := <-done
	then.Release()
	catch.Release()
	return result.value, result.err
}

func formatBytes(size int) string {
	if size >= 1<<20 {
		return fmt.Sprintf("%.1f MB", float64(size)/float64(1<<20))
	}
	return fmt.Sprintf("%d KB", (size+1023)/1024)
}

func element(tag string) js.Value {
	return js.Global().Get("document").Call("createElement", tag)
}

func textNode(value string) js.Value {
	return js.Global().Get("document").Call("createTextNode", value)
}

// swapSample replaces the editors with the starter sample for language, but
// only while they still hold the sample this playground put there. Once the
// visitor edits either editor the text is theirs, and changing grammar leaves
// it alone — switching parser to inspect your own code is the whole point.
func (h *playgroundHandle) swapSample(language string) {
	sample, ok := sampleFor(language)
	if !ok {
		return
	}
	sourceEl := h.find("#pg-source")
	queryEl := h.find("#pg-query")
	if !sourceEl.Truthy() || !queryEl.Truthy() {
		return
	}

	h.stateMu.Lock()
	seeded := h.seeded
	h.stateMu.Unlock()

	// An empty editor counts as untouched: clearing it and switching language
	// should still offer the new language's sample rather than nothing.
	currentSource := sourceEl.Get("value").String()
	if currentSource != seeded.Source && currentSource != "" {
		return
	}

	sourceEl.Set("value", sample.Source)
	// Only replace the query when it is also untouched. A visitor who wrote a
	// query but left the sample source alone keeps their query.
	if currentQuery := queryEl.Get("value").String(); currentQuery == seeded.Query || currentQuery == "" {
		queryEl.Set("value", sample.Query)
		seeded.Query = sample.Query
	}
	seeded.Source = sample.Source

	h.stateMu.Lock()
	h.seeded = seeded
	h.stateMu.Unlock()

	// Keep the highlight overlay in step with the text that just replaced it,
	// otherwise the old sample stays visible behind the transparent textarea
	// until the debounced re-highlight lands.
	h.mirrorPlainSource()
	sourceEl.Set("scrollTop", 0)
	h.syncHighlightScroll()
}

// selectSourceRange focuses the source editor and selects [start, end) in
// UTF-16 units, then scrolls the selection into view. A textarea does not
// reliably scroll to a programmatic selection on its own, so the caller-visible
// line is centred by hand.
func (h *playgroundHandle) selectSourceRange(start, end int) {
	source := h.find("#pg-source")
	if !source.Truthy() {
		return
	}
	source.Call("focus", map[string]any{"preventScroll": true})
	source.Call("setSelectionRange", start, end)

	value := source.Get("value").String()
	if start > len(value) {
		start = len(value)
	}
	line := strings.Count(value[:start], "\n")

	style := js.Global().Call("getComputedStyle", source)
	lineHeight := parseCSSPixels(style.Get("lineHeight").String())
	if lineHeight <= 0 {
		// "normal" resolves to no usable number; fall back to the font size.
		lineHeight = parseCSSPixels(style.Get("fontSize").String()) * 1.4
	}
	if lineHeight <= 0 {
		return
	}
	target := float64(line)*lineHeight - source.Get("clientHeight").Float()/2 + lineHeight
	if target < 0 {
		target = 0
	}
	source.Set("scrollTop", target)
	h.syncHighlightScroll()
}

// parseCSSPixels reads a computed CSS length such as "21.6px". It returns 0
// for keyword values like "normal", which the caller treats as "unknown".
func parseCSSPixels(value string) float64 {
	trimmed := strings.TrimSuffix(strings.TrimSpace(value), "px")
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func styledText(tag, className, value string) js.Value {
	node := element(tag)
	node.Set("className", className)
	node.Set("textContent", value)
	return node
}
