//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
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
		{"#pg-language", "change", true},
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
	if h.current(seq) {
		h.render(languageName, result, time.Since(started))
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
// exact v0.36.0 package blobs, so unbound scanners already use matching symbol
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

func (h *playgroundHandle) render(language string, result playgroundengine.Result, elapsed time.Duration) {
	h.text("#pg-language-label", language)
	h.text("#pg-node-count", fmt.Sprintf("%d nodes", result.NodeCount))
	h.text("#pg-capture-count", fmt.Sprintf("%d captures", len(result.Captures)))

	tree := h.find("#pg-tree")
	tree.Set("textContent", "")
	for _, row := range result.TreeRows {
		line := element("div")
		line.Set("className", row.Class)
		line.Set("role", "treeitem")
		line.Call("setAttribute", "aria-level", fmt.Sprint(row.Level))
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

func styledText(tag, className, value string) js.Value {
	node := element(tag)
	node.Set("className", className)
	node.Set("textContent", value)
	return node
}
