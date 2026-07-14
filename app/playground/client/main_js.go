//go:build js && wasm

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/odvcencio/gotreesitter"
	enginewasm "m31labs.dev/gosx/engine/wasm"
)

const componentName = "GoTreesitterPlayground"

func main() {
	if err := enginewasm.Register(componentName, mountPlayground); err != nil {
		panic(err)
	}
	select {}
}

type engineProps struct {
	Version           string `json:"version"`
	LanguagesURL      string `json:"languagesURL"`
	LanguageURLPrefix string `json:"languageURLPrefix"`
	DetectURL         string `json:"detectURL"`
}

type uiElements struct {
	lang      js.Value
	badge     js.Value
	dot       js.Value
	stat      js.Value
	langTag   js.Value
	highlight js.Value
	source    js.Value
	tree      js.Value
	loading   js.Value
	anon      js.Value
	query     js.Value
	queryErr  js.Value
	legend    js.Value
	queryHint js.Value
	collapse  js.Value
	queryBody js.Value
}

type listener struct {
	target js.Value
	event  string
	fn     js.Func
}

type languageSummary struct {
	Name      string `json:"name"`
	BlobBytes int    `json:"blobBytes"`
}

type languagesPayload struct {
	Version   string            `json:"version"`
	Languages []languageSummary `json:"languages"`
}

type languagePayload struct {
	Name           string `json:"name"`
	Blob           string `json:"blob"`
	HighlightQuery string `json:"highlightQuery"`
}

type detectPayload struct {
	Source string `json:"source"`
}

type detectResult struct {
	OK         bool    `json:"ok"`
	Best       string  `json:"best"`
	Confidence float64 `json:"confidence"`
}

type visibleRow struct {
	node *viewNode
	el   js.Value
}

type captureRange struct {
	start uint32
	end   uint32
	color int
}

type captureMark struct {
	name  string
	color int
}

type captureInfo struct {
	color int
	count int
}

type captureState struct {
	ranges    []captureRange
	byName    map[string]*captureInfo
	order     []string
	nodeMarks map[string][]captureMark
	truncated bool
}

type flashRange struct {
	start uint32
	end   uint32
}

type playground struct {
	root            js.Value
	abortController js.Value
	ui              uiElements
	props           engineProps

	disposeOnce sync.Once
	disposed    bool
	listeners   []listener

	ready      bool
	auto       bool
	manualLang string
	detected   string
	version    string
	loaded     map[string]*runtimeLanguage
	langIndex  map[string]languageSummary
	document   runtimeDocument

	parseSeq  uint64
	detectSeq uint64
	querySeq  uint64
	caretSeq  uint64
	flashSeq  uint64

	treeRoot       *viewNode
	visibleRows    []visibleRow
	treeFocusIndex int
	activeRow      js.Value
	renderedSource string
	hasRendered    bool
	renderedLang   string
	tokenRanges    []tokenRange
	captures       *captureState
	capturesFor    string
	flash          *flashRange
	showAnonymous  bool
	queryOpen      bool
	lastElapsed    float64
	hasElapsed     bool
	lastDot        string

	samples map[string]string

	queryProbe *queryCursorProbe
}

type queryCursorProbe struct {
	source       string
	query        *gotreesitter.Query
	cursor       *gotreesitter.QueryCursor
	nextCalls    int
	pendingMatch gotreesitter.QueryMatch
	hasPending   bool
}

func mountPlayground(engineContext enginewasm.Context) (enginewasm.Handle, error) {
	props := engineProps{}
	if err := engineContext.DecodeProps(&props); err != nil {
		return nil, fmt.Errorf("decode playground props: %w", err)
	}
	if strings.TrimSpace(props.Version) == "" {
		props.Version = "dev"
	}
	if props.LanguagesURL == "" {
		props.LanguagesURL = "/playground/langs.json"
	}
	if props.LanguageURLPrefix == "" {
		props.LanguageURLPrefix = "/playground/lang/"
	}
	if props.DetectURL == "" {
		props.DetectURL = "/playground/detect"
	}

	mount := engineContext.Mount()
	if !mount.Truthy() {
		return nil, fmt.Errorf("playground engine mount is unavailable")
	}
	root := mount.Call("querySelector", "#pg-root")
	if !root.Truthy() {
		return nil, fmt.Errorf("playground fallback root is missing")
	}
	p := &playground{
		root:      root,
		props:     props,
		auto:      true,
		detected:  "go",
		version:   props.Version,
		loaded:    make(map[string]*runtimeLanguage),
		langIndex: make(map[string]languageSummary),
		queryOpen: true,
	}
	if values, err := url.ParseQuery(strings.TrimPrefix(js.Global().Get("location").Get("search").String(), "?")); err == nil {
		if source := strings.TrimSpace(values.Get("gts-query-probe")); source != "" {
			p.queryProbe = &queryCursorProbe{source: source}
			p.root.Get("dataset").Set("queryProbeStage", "ready")
		}
	}
	if constructor := js.Global().Get("AbortController"); constructor.Type() == js.TypeFunction {
		p.abortController = constructor.New()
	} else {
		p.abortController = js.Undefined()
	}
	if err := p.bindUI(); err != nil {
		return nil, err
	}
	p.samples = map[string]string{
		"go":       p.ui.source.Get("value").String(),
		"python":   "def fib(n):\n    a, b = 0, 1\n    for _ in range(n):\n        a, b = b, a + b\n    return a\n\n\nprint([fib(i) for i in range(10)])\n",
		"json":     "{\n  \"name\": \"gotreesitter\",\n  \"grammars\": 206,\n  \"cgo\": false,\n  \"targets\": [\"js/wasm\", \"linux/arm64\"]\n}\n",
		"markdown": "# gotreesitter\n\nA **pure Go** reimplementation of tree-sitter.\n\n- 206 grammars in the box\n- zero lines of C\n\n```go\ntree, _ := parser.Parse(src)\n```\n",
	}
	p.renderPlain(p.samples["go"])
	go p.boot()
	return enginewasm.HandleFunc(p.dispose), nil
}

func (p *playground) bindUI() error {
	values := []struct {
		id   string
		dest *js.Value
	}{
		{"pg-lang", &p.ui.lang}, {"pg-badge", &p.ui.badge}, {"pg-dot", &p.ui.dot},
		{"pg-stat", &p.ui.stat}, {"pg-langtag", &p.ui.langTag}, {"pg-hl", &p.ui.highlight},
		{"pg-src", &p.ui.source}, {"pg-tree", &p.ui.tree}, {"pg-loading", &p.ui.loading},
		{"pg-anon", &p.ui.anon}, {"pg-query", &p.ui.query}, {"pg-qerr", &p.ui.queryErr},
		{"pg-legend", &p.ui.legend}, {"pg-qhint", &p.ui.queryHint},
		{"pg-qcollapse", &p.ui.collapse}, {"pg-qbd", &p.ui.queryBody},
	}
	for _, item := range values {
		value := p.root.Call("querySelector", "#"+item.id)
		if !value.Truthy() {
			return fmt.Errorf("playground element #%s is missing", item.id)
		}
		*item.dest = value
	}

	p.on(p.ui.source, "input", func(js.Value) {
		p.hasRendered = false
		p.flash = nil
		p.renderPlain(p.ui.source.Get("value").String())
		p.scheduleParse(200 * time.Millisecond)
		p.scheduleDetect()
	})
	p.on(p.ui.source, "scroll", func(js.Value) { p.syncScroll() })
	p.on(js.Global().Get("document"), "selectionchange", func(js.Value) {
		if js.Global().Get("document").Get("activeElement").Equal(p.ui.source) {
			p.scheduleCaretSync()
		}
	})
	p.on(p.ui.source, "keyup", func(js.Value) { p.scheduleCaretSync() })
	p.on(p.ui.source, "click", func(js.Value) { p.scheduleCaretSync() })
	p.on(p.ui.tree, "click", p.handleTreeClick)
	p.on(p.ui.tree, "focusin", p.handleTreeFocus)
	p.on(p.ui.tree, "keydown", p.handleTreeKeydown)
	p.on(p.ui.anon, "change", func(js.Value) {
		p.showAnonymous = p.ui.anon.Get("checked").Bool()
		nodes := p.renderTree()
		if p.hasElapsed && p.hasRendered {
			p.setStat(fmt.Sprintf("%.1f ms · %d nodes", p.lastElapsed, nodes), p.lastDot)
		}
	})
	p.on(p.ui.query, "input", func(js.Value) { p.scheduleQuery() })
	p.on(p.ui.queryHint, "click", func(js.Value) {
		p.ui.query.Set("value", `((identifier) @id (#match? @id "^ma"))`)
		p.runQuery()
		p.ui.query.Call("focus")
	})
	p.on(p.ui.collapse, "click", func(js.Value) {
		p.queryOpen = !p.queryOpen
		p.ui.queryBody.Set("hidden", !p.queryOpen)
		if p.queryOpen {
			p.ui.collapse.Set("textContent", "hide")
		} else {
			p.ui.collapse.Set("textContent", "show")
		}
		p.ui.collapse.Call("setAttribute", "aria-expanded", strconv.FormatBool(p.queryOpen))
		if p.queryOpen {
			p.runQuery()
		} else {
			p.markTreeCaptures()
			p.repaint()
		}
	})
	p.on(p.ui.lang, "change", func(js.Value) {
		value := p.ui.lang.Get("value").String()
		if value == "" {
			p.auto = true
			p.manualLang = ""
			p.scheduleParse(0)
			p.scheduleDetect()
			return
		}
		p.auto = false
		p.manualLang = value
		p.detectSeq++
		p.setBadge("")
		p.scheduleParse(0)
	})

	chips := p.root.Call("querySelectorAll", ".qchip[data-sample]")
	for index := 0; index < chips.Get("length").Int(); index++ {
		chip := chips.Index(index)
		p.on(chip, "click", func(event js.Value) {
			currentTarget := event.Get("currentTarget")
			if !currentTarget.Truthy() {
				return
			}
			name := currentTarget.Get("dataset").Get("sample").String()
			sample, ok := p.samples[name]
			if !ok {
				return
			}
			p.ui.source.Set("value", sample)
			p.ui.source.Set("scrollTop", 0)
			p.ui.source.Set("scrollLeft", 0)
			p.hasRendered = false
			p.flash = nil
			p.renderPlain(sample)
			if p.auto {
				p.setBadge("")
				p.scheduleDetect()
			}
			p.scheduleParse(0)
			if !p.ui.source.Get("disabled").Bool() {
				p.ui.source.Call("focus")
			}
		})
	}
	return nil
}

func (p *playground) on(target js.Value, event string, handler func(js.Value)) {
	fn := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if p.disposed {
			return nil
		}
		var eventValue js.Value
		if len(args) > 0 {
			eventValue = args[0]
		}
		handler(eventValue)
		return nil
	})
	target.Call("addEventListener", event, fn)
	p.listeners = append(p.listeners, listener{target: target, event: event, fn: fn})
}

func (p *playground) dispose() {
	p.disposeOnce.Do(func() {
		p.disposed = true
		if p.abortController.Truthy() {
			p.abortController.Call("abort")
		}
		p.parseSeq++
		p.detectSeq++
		p.querySeq++
		p.caretSeq++
		p.flashSeq++
		for index := len(p.listeners) - 1; index >= 0; index-- {
			item := p.listeners[index]
			item.target.Call("removeEventListener", item.event, item.fn)
			item.fn.Release()
		}
		p.listeners = nil
		p.document.close()
		p.loaded = nil
	})
}

func (p *playground) boot() {
	p.setLoading("fetching language index…")
	payload := languagesPayload{}
	if err := p.fetchJSON("GET", releaseURL(p.props.LanguagesURL, p.version), nil, &payload); err != nil {
		p.failHard(err.Error())
		return
	}
	if p.disposed {
		return
	}
	if payload.Version != "" {
		p.version = payload.Version
	}
	p.buildPicker(payload.Languages)
	p.ready = true
	p.ui.source.Set("disabled", false)
	p.ui.lang.Set("disabled", false)
	p.ui.loading.Set("hidden", true)
	p.setStat("runtime ready", "ok")
	p.runParse()
	p.scheduleDetect()
}

func (p *playground) buildPicker(languages []languageSummary) {
	document := js.Global().Get("document")
	fragment := document.Call("createDocumentFragment")
	for _, language := range languages {
		p.langIndex[language.Name] = language
		option := document.Call("createElement", "option")
		option.Set("value", language.Name)
		if language.BlobBytes > 0 {
			option.Set("textContent", language.Name+" · "+formatBytes(language.BlobBytes))
		} else {
			option.Set("textContent", language.Name+" · unavailable")
			option.Set("disabled", true)
		}
		fragment.Call("appendChild", option)
	}
	p.ui.lang.Call("appendChild", fragment)
}

func (p *playground) ensureLanguage(name string) (*runtimeLanguage, error) {
	if loaded := p.loaded[name]; loaded != nil {
		return loaded, nil
	}
	if known, ok := p.langIndex[name]; ok && known.BlobBytes == 0 {
		return nil, fmt.Errorf("%s has no compiled blob", name)
	}
	p.setStat("loading "+name+"…", "")
	payload := languagePayload{}
	endpoint := p.props.LanguageURLPrefix + url.PathEscape(name) + ".json"
	if err := p.fetchJSON("GET", releaseURL(endpoint, p.version), nil, &payload); err != nil {
		return nil, err
	}
	blob, err := base64.StdEncoding.DecodeString(payload.Blob)
	if err != nil {
		return nil, fmt.Errorf("%s: decode grammar blob: %w", name, err)
	}
	loaded, err := loadRuntimeLanguage(payload.Name, blob, payload.HighlightQuery)
	if err != nil {
		return nil, err
	}
	p.loaded[payload.Name] = loaded
	return loaded, nil
}

func (p *playground) fetchJSON(method, endpoint string, body any, target any) error {
	options := js.Global().Get("Object").New()
	options.Set("method", method)
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		headers := js.Global().Get("Object").New()
		headers.Set("Content-Type", "application/json")
		options.Set("headers", headers)
		options.Set("body", string(encoded))
	}
	if p.abortController.Truthy() {
		options.Set("signal", p.abortController.Get("signal"))
	}
	response, err := awaitPromise(js.Global().Call("fetch", endpoint, options))
	if err != nil {
		return err
	}
	if !response.Get("ok").Bool() {
		return fmt.Errorf("%s failed: HTTP %d", endpoint, response.Get("status").Int())
	}
	payload, err := awaitPromise(response.Call("json"))
	if err != nil {
		return fmt.Errorf("decode %s: %w", endpoint, err)
	}
	encoded := js.Global().Get("JSON").Call("stringify", payload)
	if encoded.Type() != js.TypeString {
		return fmt.Errorf("decode %s: response is not JSON", endpoint)
	}
	if err := json.Unmarshal([]byte(encoded.String()), target); err != nil {
		return fmt.Errorf("decode %s: %w", endpoint, err)
	}
	return nil
}

type promiseResult struct {
	value js.Value
	err   error
}

func awaitPromise(promise js.Value) (js.Value, error) {
	result := make(chan promiseResult, 1)
	resolve := js.FuncOf(func(_ js.Value, args []js.Value) any {
		value := js.Undefined()
		if len(args) > 0 {
			value = args[0]
		}
		result <- promiseResult{value: value}
		return nil
	})
	reject := js.FuncOf(func(_ js.Value, args []js.Value) any {
		message := "promise rejected"
		if len(args) > 0 {
			if property := args[0].Get("message"); property.Type() == js.TypeString {
				message = property.String()
			} else {
				message = args[0].String()
			}
		}
		result <- promiseResult{err: fmt.Errorf("%s", message)}
		return nil
	})
	promise.Call("then", resolve, reject)
	settled := <-result
	resolve.Release()
	reject.Release()
	return settled.value, settled.err
}

func (p *playground) scheduleParse(delay time.Duration) {
	p.parseSeq++
	sequence := p.parseSeq
	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}
		if p.disposed || sequence != p.parseSeq {
			return
		}
		p.runParse()
	}()
}

func (p *playground) runParse() {
	if !p.ready || p.disposed {
		return
	}
	p.parseSeq++
	sequence := p.parseSeq
	languageName := p.manualLang
	if p.auto {
		if guess := heuristicDetect(p.ui.source.Get("value").String()); guess != "" && guess != p.detected {
			p.detected = guess
			p.setBadge("detected " + guess + " · signature")
		}
		languageName = p.detected
	}
	if languageName == "" {
		return
	}
	loaded, err := p.ensureLanguage(languageName)
	if err != nil {
		if !p.disposed {
			p.setStat(err.Error(), "err")
		}
		return
	}
	if p.disposed || sequence != p.parseSeq {
		return
	}
	source := p.ui.source.Get("value").String()
	started := time.Now()
	if p.document.tree == nil || p.document.language != loaded {
		err = p.document.open(loaded, source)
	} else {
		err = p.document.update(source)
	}
	elapsed := float64(time.Since(started).Microseconds()) / 1000
	if err != nil {
		p.setStat("parse failed: "+err.Error(), "err")
		return
	}
	root := p.document.tree.RootNode()
	p.treeRoot = buildViewTree(p.document.tree, loaded.language, maxTreeNodes)
	p.renderedSource = source
	p.hasRendered = true
	p.renderedLang = languageName
	p.tokenRanges = documentTokenRanges(&p.document)
	p.flash = nil
	p.lastElapsed = elapsed
	p.hasElapsed = true
	p.lastDot = "ok"
	if root != nil && root.HasError() {
		p.lastDot = "err"
	}
	label := languageName
	if p.auto {
		label += " · auto"
	}
	p.ui.langTag.Set("textContent", label)
	nodes := p.renderTree()
	p.repaint()
	if p.queryOpen && strings.TrimSpace(p.ui.query.Get("value").String()) != "" {
		p.runQuery()
	}
	p.setStat(fmt.Sprintf("%.1f ms · %d nodes", elapsed, nodes), p.lastDot)
	p.root.Get("dataset").Set("language", languageName)
	p.root.Get("dataset").Set("revision", strconv.FormatUint(p.document.revision, 10))
}

func (p *playground) scheduleDetect() {
	if !p.auto {
		return
	}
	p.detectSeq++
	sequence := p.detectSeq
	go func() {
		time.Sleep(600 * time.Millisecond)
		if p.disposed || !p.auto || sequence != p.detectSeq {
			return
		}
		p.runDetect(sequence)
	}()
}

func (p *playground) runDetect(sequence uint64) {
	if !p.auto || !p.ready || p.disposed {
		return
	}
	source := p.ui.source.Get("value").String()
	if strings.TrimSpace(source) == "" {
		return
	}
	payload := detectResult{}
	if err := p.fetchJSON("POST", p.props.DetectURL, detectPayload{Source: boundedSource(source, 4096)}, &payload); err != nil {
		return
	}
	if p.disposed || sequence != p.detectSeq || !p.auto || !payload.OK || payload.Best == "" {
		return
	}
	label := "low"
	if payload.Confidence >= 0.75 {
		label = "high"
	} else if payload.Confidence >= 0.5 {
		label = "medium"
	}
	p.setBadge(fmt.Sprintf("detected %s · %s %.0f%%", payload.Best, label, payload.Confidence*100))
	if payload.Best != p.detected && payload.Confidence >= 0.5 {
		p.detected = payload.Best
		p.scheduleParse(0)
	}
}

func (p *playground) scheduleQuery() {
	p.querySeq++
	sequence := p.querySeq
	go func() {
		time.Sleep(250 * time.Millisecond)
		if p.disposed || sequence != p.querySeq {
			return
		}
		p.runQuery()
	}()
}

// runQueryProbe advances exactly one public query-cursor operation per user
// event. Splitting compilation, cursor creation, and each NextMatch call across
// browser turns makes a WASM realm stall attributable without authored JS or a
// timer that could hide the blocking operation.
func (p *playground) runQueryProbe() {
	probe := p.queryProbe
	if probe == nil || p.document.tree == nil || p.document.language == nil {
		return
	}
	dataset := p.root.Get("dataset")
	if probe.query == nil {
		dataset.Set("queryProbeStage", "new-query-started")
		query, err := gotreesitter.NewQuery(probe.source, p.document.language.language)
		if err != nil {
			dataset.Set("queryProbeStage", "new-query-error")
			return
		}
		probe.query = query
		dataset.Set("queryProbeStage", "new-query-returned")
		return
	}
	if probe.cursor == nil {
		dataset.Set("queryProbeStage", "exec-started")
		probe.cursor = probe.query.Exec(
			p.document.tree.RootNode(),
			p.document.language.language,
			p.document.tree.Source(),
		)
		probe.cursor.SetMatchLimit(maxQueryMatches)
		dataset.Set("queryProbeStage", "exec-returned")
		return
	}
	if probe.hasPending {
		dataset.Set("queryProbeStage", fmt.Sprintf("captures-%d-started", probe.nextCalls))
		for _, capture := range probe.pendingMatch.Captures {
			if capture.Node == nil {
				continue
			}
			_ = capture.Node.Type(p.document.language.language)
			_, _ = p.document.tree.UTF16RangeForNode(capture.Node)
		}
		probe.pendingMatch = gotreesitter.QueryMatch{}
		probe.hasPending = false
		dataset.Set("queryProbeStage", fmt.Sprintf("captures-%d-returned", probe.nextCalls))
		return
	}
	probe.nextCalls++
	dataset.Set("queryProbeStage", fmt.Sprintf("next-%d-started", probe.nextCalls))
	match, ok := probe.cursor.NextMatch()
	if !ok {
		dataset.Set("queryProbeStage", fmt.Sprintf("next-%d-done", probe.nextCalls))
		return
	}
	probe.pendingMatch = match
	probe.hasPending = true
	dataset.Set("queryProbeCaptures", strconv.Itoa(len(match.Captures)))
	dataset.Set("queryProbeStage", fmt.Sprintf("next-%d-returned", probe.nextCalls))
}

func (p *playground) scheduleCaretSync() {
	p.caretSeq++
	sequence := p.caretSeq
	go func() {
		time.Sleep(120 * time.Millisecond)
		if p.disposed || sequence != p.caretSeq || !p.hasRendered {
			return
		}
		if !js.Global().Get("document").Get("activeElement").Equal(p.ui.source) {
			return
		}
		p.highlightTreeNode(p.deepestVisibleAt(p.ui.source.Get("selectionStart").Int()))
	}()
}

func (p *playground) setLoading(text string) {
	if p.disposed {
		return
	}
	p.ui.loading.Set("hidden", false)
	p.ui.loading.Get("classList").Call("remove", "err")
	p.ui.loading.Set("textContent", text)
}

func (p *playground) failHard(message string) {
	if p.disposed {
		return
	}
	p.ui.loading.Set("hidden", false)
	p.ui.loading.Get("classList").Call("add", "err")
	p.ui.loading.Set("textContent", message)
	p.ui.stat.Set("textContent", "runtime unavailable")
	p.ui.dot.Set("className", "pg-dot err")
}

func (p *playground) setStat(text, class string) {
	if p.disposed {
		return
	}
	p.ui.stat.Set("textContent", text)
	dotClass := "pg-dot"
	if class != "" {
		dotClass += " " + class
	}
	p.ui.dot.Set("className", dotClass)
}

func (p *playground) setBadge(text string) {
	if text == "" {
		p.ui.badge.Set("hidden", true)
		return
	}
	p.ui.badge.Set("hidden", false)
	p.ui.badge.Set("textContent", text)
}

func releaseURL(endpoint, version string) string {
	separator := "?"
	if strings.Contains(endpoint, "?") {
		separator = "&"
	}
	return endpoint + separator + "v=" + url.QueryEscape(version)
}

func formatBytes(size int) string {
	if size >= 1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%d KB", max(1, (size+512)/1024))
}

func boundedSource(source string, size int) string {
	if len(source) <= size {
		return source
	}
	return strings.ToValidUTF8(source[:size], "")
}
