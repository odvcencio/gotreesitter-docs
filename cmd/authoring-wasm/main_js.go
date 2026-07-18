//go:build js && wasm

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/odvcencio/gotreesitter-docs/internal/authoringengine"
	enginewasm "m31labs.dev/gosx/engine/wasm"
)

// Phase 1 architecture: this GoSX island (runtime="go-wasm") keeps owning
// the DOM — props, listeners, rendering — exactly like Phase 0. What moved
// is the compute: grammargen's import->generate->diagnose->highlight->parse
// loop now runs in a dedicated Web Worker (cmd/authoring-worker-wasm, a
// plain js/wasm build, spawned here via syscall/js's Worker constructor —
// see spawnWorker) instead of inline on this thread. That is the "clean
// path" for a grammar-authoring compiler in this codebase:
//   - GoSX's engine/wasm island runtime (this package) is a DOM-binding
//     abstraction; its Context has no meaning inside a Worker's global
//     scope (no document, no mount).
//   - GoSX v0.31.4 does define an engine.KindWorker ("background compute,
//     no DOM"), reachable from .gsx via <Worker runtime="go-wasm" ...>, but
//     nothing in the framework's own client bootstrap (checked in
//     build/bootstrap-feature-engines.js — no `new Worker(` anywhere in the
//     module) actually spawns a browser Worker thread for it; it is a
//     same-thread engine that merely skips mounting a DOM element. Using it
//     would not solve the actual risk (a heavy grammar still freezes the
//     tab), so it was not used here.
//   - What actually offloads compute is a real `new Worker(...)` created by
//     this file, running the wasm/grammargen-style plain build in
//     cmd/authoring-worker-wasm.
//
// Message protocol, seq gating, and the watchdog/respawn safety net are
// documented on the fields/methods below.
type browserListener struct {
	target js.Value
	event  string
	fn     js.Func
}

const (
	debounceDelay = 180 * time.Millisecond
	// softBudget/hardBudget bound how long the background worker gets before
	// the UI, respectively, warns the author and gives up on it. See
	// cmd/authoring-worker-wasm's generationBudget doc for why this
	// main-thread watchdog — not the worker's own context timeout — is the
	// real backstop: a single-threaded Go/wasm goroutine in a tight
	// computational loop cannot be preempted mid-flight by anything short of
	// terminating the Worker outright.
	//
	// Pre-deploy hardening pass (2026-07): sized off a REAL worker-side
	// (wasm) measurement, not native numbers or a guess — the previous
	// hardBudget (15s) was set from grammargen.LoxGrammar()'s native
	// generate+diagnose cost (~1.7s combined) times a rule-of-thumb "Go/wasm
	// runs 2-4x slower" multiplier, which undersold the real cost badly
	// enough that the headline "inherit Go" base itself tripped the watchdog
	// under ordinary host load. cmd/verify-authoring-browser (run against a
	// real headless-Chrome worker, through the full gosx build --prod
	// pipeline, on a moderately loaded dev machine — load average ~5-6 on 20
	// cores, not idle) measured the Go base's own self-reported
	// generate+diagnose+parse time (authoringengine.CompileWithContext,
	// which pays for LR generation twice — see authoringengine's diagnose
	// doc) at 12.4s-14.5s across five independent compiles, zero crashes.
	// hardBudget is set to comfortably exceed that (~2.4-2.8x the observed
	// range) so a normal-load Go compile never trips the kill, while still
	// bounding a genuinely pathological custom grammar to a reasonable wait
	// before the terminate+respawn backstop (onHardTimeout) kicks in.
	//
	// The same measurement pass is why typescript/javascript are NOT in
	// public/authoring/bases/index.json even though grammargen ships
	// TypescriptGrammar/JavascriptGrammar: typescript's worker-side compile
	// crashed the Worker's own Go runtime outright ("Go program has already
	// exited" — almost certainly the wasm linear-memory arena being
	// exhausted, not a mere timeout) in two independent, isolated
	// (fresh-page) measurements; javascript did not crash but was still
	// running past 180s (no completion, no timeout, no crash — just far
	// outside any interactive budget) in an isolated measurement. Raising
	// hardBudget cannot fix either: one is a crash, and the other would
	// require a multi-minute budget that defeats the watchdog's purpose as a
	// backstop for pathological CUSTOM grammars too. See
	// cmd/build-authoring-wasm's baseGrammars doc for the full base-by-base
	// decision.
	//
	// softBudget fires a "still working" status well before hardBudget so a
	// legitimately heavy compile (Go, lox) shows signs of life instead of
	// looking frozen; 2-3s is early enough to reassure the author without
	// being noisy on every merely-non-instant grammar.
	softBudget = 3 * time.Second
	hardBudget = 35 * time.Second
	// bootBudget bounds how long a newly spawned worker has to send its
	// "ready" ping (importScripts + fetch + WebAssembly.instantiate +
	// go.run for a ~17MB binary) before a pending request falls back to the
	// main thread. onerror already covers most startup failures; this is
	// the backstop for the rest (e.g. a fetch that hangs without erroring).
	bootBudget = 20 * time.Second

	// defaultBaseName is selected automatically once bases/index.json loads,
	// so the very first compile on page load already demonstrates
	// inheritance (a base + the seed delta — see app/authoring/page.server.go)
	// rather than requiring a click. calc is grammargen's fastest base
	// (sub-millisecond native generation — see cmd/build-authoring-wasm's
	// baseGrammars doc), so this stays instant even before the worker
	// finishes booting (the local main-thread fallback path, or the worker
	// once ready, both complete essentially immediately).
	defaultBaseName = "calc"
)

// baseAsset mirrors cmd/build-authoring-wasm's baseAsset — one entry from
// public/authoring/bases/index.json (a pre-generated grammar.json base the
// author can inherit from; see internal/authoringengine.MergeGrammarJSON).
type baseAsset struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	RuleCount int    `json:"ruleCount"`
}

type authoringProps struct {
	WorkerScriptURL string `json:"workerScriptURL"`
	// BaseIndexURL points at public/authoring/bases/index.json. Empty (e.g.
	// a future embed that ships no base assets) falls back to Phase 0/1's
	// "blank / full grammar" behavior only — #ag-grammar is the whole
	// grammar, no base picker is populated.
	BaseIndexURL string `json:"baseIndexURL"`
}

type authoringHandle struct {
	mount     js.Value
	listeners []browserListener

	stateMu  sync.Mutex
	timer    *time.Timer
	disposed bool
	// runSeq is bumped on every debounced edit (or immediate toggle) and is
	// the single source of truth for "is this the latest request" — both
	// for gating worker responses and for the local-fallback goroutine path.
	runSeq uint64

	workerScriptURL string
	worker          js.Value
	workerFuncs     []js.Func
	workerBroken    bool
	// workerReady flips true once the worker's own "ready" ping is received.
	// A Worker's script loads asynchronously (importScripts + fetch +
	// WebAssembly.instantiate + go.run, easily hundreds of ms for a ~17MB
	// binary), so self.onmessage isn't registered inside the worker until
	// well after `new Worker(url)` returns here. Empirically (verified
	// against this exact build with a chromedp harness instrumenting
	// Worker.prototype.postMessage): a message posted synchronously right
	// after construction — i.e. exactly what mount->run->dispatch used to
	// do — is silently dropped; the worker's very first script-eval task
	// finishes (the synchronous importScripts + fetch()-kickoff) and hands
	// control back to its event loop before go.run() ever gets to register
	// self.onmessage, so the queued "message" event dispatches to no
	// listener and is lost, not re-queued. So: never postMessage before
	// ready; queue at most one pendingRequest and flush it once ready
	// arrives (a message posted after ready arrives back on the main
	// thread is reliably delivered — the worker is by then idle in
	// select{}, actively listening).
	workerReady bool
	pendingReq  *authoringRequest
	bootTimer   *time.Timer
	// workerGen changes every time the worker is (re)spawned so a stray
	// message/error callback from a just-terminated worker can't touch state
	// for its successor.
	workerGen uint64

	// inFlightSeq is the seq currently posted to the worker awaiting a
	// response (0 = idle). lastSource/lastGrammarJSON are the sample text
	// and the (possibly base+delta merged) grammar.json sent alongside it,
	// snapshotted so the highlight renderer — and, for lastGrammarJSON,
	// render's export-readiness bookkeeping (see canExport) — use exactly
	// the bytes the worker actually compiled, even if the textarea has
	// since been edited again (that edit would have already produced a
	// newer, non-matching runSeq, so this response would be dropped as
	// stale before either field is ever read for it).
	inFlightSeq     uint64
	lastSource      string
	lastGrammarJSON string
	softTimer       *time.Timer
	hardTimer       *time.Timer

	// Phase 2 inheritance state. baseIndexURL/bases come from
	// public/authoring/bases/index.json (fetched once, at mount — see
	// bootstrapBases). baseCache holds fetched base grammar.json bytes keyed
	// by name so switching back and forth between bases the author already
	// visited doesn't re-fetch. All three are guarded by stateMu like every
	// other field above; there is no separate lock because base changes are
	// rare (a select's change event, not a per-keystroke event) and always
	// funnel through the same run()/dispatch() path as an ordinary edit.
	baseIndexURL string
	bases        []baseAsset
	baseCache    map[string][]byte

	// Phase 3 export state. lastGrammarJSON/lastSource above already
	// snapshot the request each response corresponds to; canExport/
	// exportGrammarJSON/exportName are the subset of that identified as
	// "the most recent SUCCESSFUL compile" — see render's success branch —
	// which is what onExportClick actually sends to the worker/local
	// fallback. exportBusy tracks per-format in-flight export requests so a
	// double-click can't fire two overlapping downloads for the same
	// format, and exportSeq is a monotonically increasing id included on
	// the wire purely for protocol symmetry with compile's seq (export
	// responses are not superseded/dropped the way a stale compile response
	// is — every button click's own response is honored whenever it
	// arrives).
	canExport         bool
	exportGrammarJSON string
	exportName        string
	exportSeq         uint64
	exportBusy        map[string]bool
}

// authoringRequest is one debounced edit waiting to be sent to the worker —
// either because the worker isn't ready yet (see workerReady above) or, in
// principle, any other reason dispatch decides to defer it.
type authoringRequest struct {
	seq              uint64
	grammarJSON      string
	source           string
	includeAnonymous bool
}

func main() {
	if err := enginewasm.Register("GotreesitterAuthoring", mountAuthoring); err != nil {
		panic(err)
	}
	select {}
}

// mountAuthoring wires the grammar-authoring surface: a base-grammar picker
// (#ag-base), a grammar.json/delta textarea (#ag-grammar), a sample-source
// textarea (#ag-source), an anonymous-node toggle (#ag-anonymous), an
// optional rename field (#ag-name), and the render sinks
// (#ag-tree/#ag-errors/#ag-status/#ag-node-count/#ag-conflicts/
// #ag-warnings/#ag-highlight/#ag-base-info/#ag-fidelity-note/
// #ag-editor-label). It spawns the background compiler Worker immediately so
// the very first (seed) compile already goes through it, and — if
// BaseIndexURL is configured — fetches public/authoring/bases/index.json to
// populate the base picker; that fetch's completion is what actually kicks
// off the first compile (see bootstrapBases), so on a base-enabled page the
// unconditional h.run() Phase 0/1 called here would be redundant (and would
// dispatch against #ag-base's not-yet-populated "" value): only used when
// BaseIndexURL is unset (blank/full-grammar-only mode).
func mountAuthoring(ctx enginewasm.Context) (enginewasm.Handle, error) {
	mount := ctx.Mount()
	if !mount.Truthy() {
		return nil, fmt.Errorf("authoring surface requires a DOM mount")
	}
	h := &authoringHandle{mount: mount, baseCache: map[string][]byte{}, exportBusy: map[string]bool{}}
	var props authoringProps
	if err := ctx.DecodeProps(&props); err == nil {
		h.workerScriptURL = strings.TrimSpace(props.WorkerScriptURL)
		h.baseIndexURL = strings.TrimSpace(props.BaseIndexURL)
	}

	for _, binding := range []struct {
		selector  string
		event     string
		immediate bool
	}{
		{"#ag-grammar", "input", false},
		{"#ag-source", "input", false},
		{"#ag-anonymous", "change", true},
		{"#ag-name", "input", false},
	} {
		target := h.find(binding.selector)
		if !target.Truthy() {
			return nil, fmt.Errorf("authoring element %s is missing", binding.selector)
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

	// #ag-base gets its own listener (not the generic run()/schedule() loop
	// above): picking a new base needs to fetch that base's grammar.json
	// before a merge+compile can run at all — see onBaseChange.
	baseSelect := h.find("#ag-base")
	if !baseSelect.Truthy() {
		return nil, fmt.Errorf("authoring element #ag-base is missing")
	}
	baseChangeFn := js.FuncOf(func(js.Value, []js.Value) any {
		h.onBaseChange()
		return nil
	})
	baseSelect.Call("addEventListener", "change", baseChangeFn)
	h.listeners = append(h.listeners, browserListener{target: baseSelect, event: "change", fn: baseChangeFn})

	if err := h.bindExportButtons(); err != nil {
		return nil, err
	}
	h.updateExportButtons(false) // no successful compile yet

	h.mount.Get("dataset").Set("privacyBoundary", "browser-only")

	if h.workerScriptURL == "" {
		h.workerBroken = true
	} else {
		h.spawnWorker(0)
	}
	if h.baseIndexURL != "" {
		go h.bootstrapBases()
	} else {
		h.run()
	}
	return h, nil
}

func (h *authoringHandle) Dispose() {
	h.stateMu.Lock()
	h.disposed = true
	h.runSeq++
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	h.disarmWatchdogLocked()
	if h.bootTimer != nil {
		h.bootTimer.Stop()
		h.bootTimer = nil
	}
	worker := h.worker
	funcs := h.workerFuncs
	h.worker = js.Undefined()
	h.workerFuncs = nil
	h.stateMu.Unlock()

	for _, listener := range h.listeners {
		listener.target.Call("removeEventListener", listener.event, listener.fn)
		listener.fn.Release()
	}
	h.listeners = nil

	for _, fn := range funcs {
		fn.Release()
	}
	if worker.Truthy() {
		worker.Call("terminate")
	}
}

func (h *authoringHandle) schedule() {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	if h.disposed {
		return
	}
	if h.timer != nil {
		h.timer.Stop()
	}
	h.timer = time.AfterFunc(debounceDelay, h.run)
}

// run reads the current editor state and dispatches it — to the worker if
// one is available, otherwise to a local (main-thread) fallback so the page
// still works without Worker support. It is safe to call directly (mount's
// first compile, the anonymous-toggle's immediate path, and the debounce
// timer all do).
//
// Phase 2 inheritance: when #ag-base names a base, #ag-grammar is treated as
// a delta and merged with that base's cached grammar.json — entirely on this
// thread, entirely in Go, via authoringengine.MergeGrammarJSON — before
// anything is dispatched. The worker's wire protocol is unchanged from Phase
// 1: it only ever receives one already-complete grammarJSON string and has
// no notion of "base" or "delta" (see design doc Phase 2 item 4 — "the
// merged grammar.json goes to the Phase 1 worker exactly as the full grammar
// did"). When #ag-base is "" (the "blank / full grammar" option), #ag-grammar
// is dispatched unmerged, exactly as in Phase 0/1.
func (h *authoringHandle) run() {
	h.stateMu.Lock()
	if h.disposed {
		h.stateMu.Unlock()
		return
	}
	h.timer = nil
	h.runSeq++
	seq := h.runSeq
	h.stateMu.Unlock()

	grammarOrDelta := h.find("#ag-grammar").Get("value").String()
	source := h.find("#ag-source").Get("value").String()
	includeAnonymous := true
	if el := h.find("#ag-anonymous"); el.Truthy() {
		includeAnonymous = el.Get("checked").Bool()
	}
	baseName := ""
	if el := h.find("#ag-base"); el.Truthy() {
		baseName = el.Get("value").String()
	}
	nameOverride := ""
	if el := h.find("#ag-name"); el.Truthy() {
		nameOverride = strings.TrimSpace(el.Get("value").String())
	}

	if baseName == "" {
		h.updateFidelityNote("", "")
		h.dispatch(seq, grammarOrDelta, source, includeAnonymous)
		return
	}

	h.stateMu.Lock()
	baseBytes, ok := h.baseCache[baseName]
	h.stateMu.Unlock()
	if !ok {
		// The base picker's change handler (onBaseChange/loadBaseAndRun) is
		// still fetching this base; it calls run() again once the fetch
		// lands. Nothing to dispatch yet.
		h.text("#ag-status", fmt.Sprintf("Loading base grammar %q…", baseName))
		return
	}

	h.updateFidelityNote(baseName, nameOverride)
	merged, err := authoringengine.MergeGrammarJSON(baseBytes, []byte(grammarOrDelta), nameOverride)
	if err != nil {
		if h.current(seq) {
			// The merge itself failed before anything was ever dispatched —
			// there is no grammarJSON to remember as exportable.
			h.render("", "", authoringengine.Result{ImportError: "delta grammar.json: " + err.Error()}, 0, "")
		}
		return
	}
	h.dispatch(seq, string(merged), source, includeAnonymous)
}

// bootstrapBases fetches public/authoring/bases/index.json, populates
// #ag-base with one <option> per entry, and selects defaultBaseName so the
// very first compile already demonstrates inheritance. Runs once, from a
// goroutine kicked off by mountAuthoring.
func (h *authoringHandle) bootstrapBases() {
	var assets []baseAsset
	if err := fetchJSON(h.baseIndexURL, &assets); err != nil {
		h.text("#ag-base-info", "Could not load the base grammar index: "+err.Error())
		return
	}
	h.stateMu.Lock()
	if h.disposed {
		h.stateMu.Unlock()
		return
	}
	h.bases = assets
	h.stateMu.Unlock()

	sel := h.find("#ag-base")
	if !sel.Truthy() {
		return
	}
	for _, asset := range assets {
		option := element("option")
		option.Set("value", asset.Name)
		option.Set("textContent", fmt.Sprintf("%s (%d rules)", asset.Name, asset.RuleCount))
		sel.Call("appendChild", option)
	}
	h.mount.Get("dataset").Set("baseCount", fmt.Sprint(len(assets)))

	if _, ok := h.findBaseAsset(defaultBaseName); ok {
		sel.Set("value", defaultBaseName)
	}
	h.onBaseChange()
}

// onBaseChange runs whenever #ag-base's selection changes (including the
// synthetic call bootstrapBases makes once it has picked defaultBaseName).
// A base switch always needs an async fetch (or a cache hit) before a
// merge+compile can happen, so — unlike every other input on this page — it
// does not go through schedule()/run() directly.
func (h *authoringHandle) onBaseChange() {
	sel := h.find("#ag-base")
	name := ""
	if sel.Truthy() {
		name = sel.Get("value").String()
	}
	h.updateEditorLabel(name)
	if name == "" {
		h.text("#ag-base-info", "")
		h.updateFidelityNote("", "")
		h.run()
		return
	}
	h.stateMu.Lock()
	_, cached := h.baseCache[name]
	h.stateMu.Unlock()
	if cached {
		h.describeCachedBase(name)
		h.run()
		return
	}
	h.text("#ag-base-info", fmt.Sprintf("Loading base grammar %q…", name))
	go h.loadBaseAndRun(name)
}

// loadBaseAndRun fetches (and caches) the named base's grammar.json, then
// calls run() so the merge+compile that was waiting on it proceeds.
func (h *authoringHandle) loadBaseAndRun(name string) {
	asset, ok := h.findBaseAsset(name)
	if !ok {
		h.text("#ag-base-info", "Unknown base grammar "+name)
		return
	}
	data, err := fetchBytes(asset.URL)
	if err != nil {
		h.text("#ag-base-info", fmt.Sprintf("Could not load base %q: %s", name, err))
		return
	}
	h.stateMu.Lock()
	disposed := h.disposed
	if !disposed {
		if h.baseCache == nil {
			h.baseCache = map[string][]byte{}
		}
		h.baseCache[name] = data
	}
	h.stateMu.Unlock()
	if disposed {
		return
	}
	h.describeCachedBase(name)
	h.run()
}

// describeCachedBase updates #ag-base-info from an already-fetched (cached)
// base's grammar.json — the "Base “go” — 116 rules" line that tells the
// author what they're inheriting (design Phase 2 item 2).
func (h *authoringHandle) describeCachedBase(name string) {
	h.stateMu.Lock()
	data := h.baseCache[name]
	h.stateMu.Unlock()
	baseName, ruleCount, err := authoringengine.GrammarSummary(data)
	if err != nil {
		h.text("#ag-base-info", fmt.Sprintf("Base %q loaded but failed to parse: %s", name, err))
		return
	}
	h.text("#ag-base-info", fmt.Sprintf(
		"Base %q — %d rule%s. The editor below is your delta: add new rules or override any of these; everything else is inherited unchanged.",
		baseName, ruleCount, plural(ruleCount),
	))
}

func (h *authoringHandle) findBaseAsset(name string) (baseAsset, bool) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	for _, asset := range h.bases {
		if asset.Name == name {
			return asset, true
		}
	}
	return baseAsset{}, false
}

// updateEditorLabel relabels the delta/grammar.json panel's heading so it's
// honest about what #ag-grammar means in the current mode (design Phase 2
// item 2: "reframe/relabel the existing grammar textarea as the DELTA").
func (h *authoringHandle) updateEditorLabel(baseName string) {
	label := h.find("#ag-editor-label")
	if !label.Truthy() {
		return
	}
	if baseName == "" {
		label.Set("textContent", "grammar.json")
	} else {
		label.Set("textContent", fmt.Sprintf("delta — rules added to / overridden on %q", baseName))
	}
}

// updateFidelityNote surfaces design risk #3 (shape-hint fidelity): renaming
// an inherited base can shift grammargen's few name-keyed compiler behaviors.
// See internal/authoringengine's MergeGrammarJSON doc for exactly what is and
// isn't carried through a rename. Empty baseName or an unchanged/empty
// nameOverride clears the note.
func (h *authoringHandle) updateFidelityNote(baseName, nameOverride string) {
	note := h.find("#ag-fidelity-note")
	if !note.Truthy() {
		return
	}
	if baseName == "" || nameOverride == "" || nameOverride == baseName {
		note.Set("textContent", "")
		return
	}
	note.Set("textContent", fmt.Sprintf(
		"Renamed %q → %q: this build carries the base's already-resolved LR-tuning flags into the merged grammar, so most inherited grammars are unaffected — but grammargen has a small number of deeper, name-keyed compiler behaviors this data-only merge can't re-target, so conflict resolution could still shift for edge cases until a Go-side ExtendGrammarJSON exists.",
		baseName, nameOverride,
	))
}

func (h *authoringHandle) dispatch(seq uint64, grammarJSON, source string, includeAnonymous bool) {
	h.stateMu.Lock()
	useWorker := !h.workerBroken && h.worker.Truthy()
	ready := h.workerReady
	h.stateMu.Unlock()

	if !useWorker {
		h.text("#ag-status", "Compiling locally on the main thread (background worker unavailable)…")
		go h.runLocalFallback(seq, grammarJSON, source, includeAnonymous)
		return
	}

	req := &authoringRequest{seq: seq, grammarJSON: grammarJSON, source: source, includeAnonymous: includeAnonymous}
	if !ready {
		// See workerReady's doc: posting before the worker's own onmessage
		// listener is attached silently loses the message, so hold the
		// latest edit and let handleWorkerMessage's "ready" branch send it.
		h.stateMu.Lock()
		h.pendingReq = req
		h.stateMu.Unlock()
		h.text("#ag-status", "Starting the background compiler…")
		return
	}
	h.postToWorker(req)
}

// postToWorker actually sends req to a worker already known to be ready
// (workerReady == true at the time the caller checked).
func (h *authoringHandle) postToWorker(req *authoringRequest) {
	h.stateMu.Lock()
	h.inFlightSeq = req.seq
	h.lastSource = req.source
	h.lastGrammarJSON = req.grammarJSON
	worker := h.worker
	h.stateMu.Unlock()

	h.armWatchdog(req.seq)
	h.text("#ag-status", "Compiling in a background worker…")
	worker.Call("postMessage", js.ValueOf(map[string]any{
		"seq":              float64(req.seq),
		"grammarJSON":      req.grammarJSON,
		"source":           req.source,
		"includeAnonymous": req.includeAnonymous,
	}))
}

// runLocalFallback mirrors Phase 0's inline compile path. It only runs when
// the Worker could not be created (unsupported browser, blocked script, or a
// worker-side crash observed via onerror).
func (h *authoringHandle) runLocalFallback(seq uint64, grammarJSON, source string, includeAnonymous bool) {
	defer func() {
		if recovered := recover(); recovered != nil && h.current(seq) {
			h.fail(fmt.Sprintf("browser compiler failed: %v", recovered))
		}
	}()
	started := time.Now()
	result := authoringengine.CompileWithContext(context.Background(), grammarJSON, source, includeAnonymous)
	if h.current(seq) {
		h.render(grammarJSON, source, result, time.Since(started), "main-thread")
	}
}

func (h *authoringHandle) current(seq uint64) bool {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	return !h.disposed && h.runSeq == seq
}

// spawnWorker creates the background compiler Worker for generation gen (the
// value of h.workerGen at the time of the call, or 0 for the initial spawn
// at mount). Every callback closed over here checks that h.workerGen is
// still gen before touching state, so a message from a Worker we've already
// terminated (via the hard-timeout respawn) is inert.
func (h *authoringHandle) spawnWorker(gen uint64) {
	ctor := js.Global().Get("Worker")
	if ctor.Type() != js.TypeFunction {
		h.stateMu.Lock()
		h.workerBroken = true
		h.stateMu.Unlock()
		h.text("#ag-status", "This browser has no Web Worker support; compiling on the main thread instead.")
		return
	}

	var worker js.Value
	created := func() (ok bool) {
		defer func() {
			if recover() != nil {
				ok = false
			}
		}()
		worker = ctor.New(h.workerScriptURL)
		return worker.Truthy()
	}()
	if !created {
		h.stateMu.Lock()
		h.workerBroken = true
		h.stateMu.Unlock()
		h.text("#ag-status", "Could not start the background compiler; compiling on the main thread instead.")
		return
	}

	onMessage := js.FuncOf(func(_ js.Value, args []js.Value) any {
		h.handleWorkerMessage(gen, args)
		return nil
	})
	onError := js.FuncOf(func(_ js.Value, args []js.Value) any {
		h.stateMu.Lock()
		if h.disposed || h.workerGen != gen {
			h.stateMu.Unlock()
			return nil
		}
		h.workerBroken = true
		pending := h.pendingReq
		h.pendingReq = nil
		h.stateMu.Unlock()
		message := "background compiler crashed"
		if len(args) > 0 {
			if m := args[0].Get("message"); m.Type() == js.TypeString && m.String() != "" {
				message = "background compiler crashed: " + m.String()
			}
		}
		h.text("#ag-status", message+"; falling back to the main thread.")
		if pending != nil {
			go h.runLocalFallback(pending.seq, pending.grammarJSON, pending.source, pending.includeAnonymous)
		}
		return nil
	})
	worker.Set("onmessage", onMessage)
	worker.Set("onerror", onError)

	h.stateMu.Lock()
	h.worker = worker
	h.workerBroken = false
	h.workerReady = false
	h.workerFuncs = []js.Func{onMessage, onError}
	if h.bootTimer != nil {
		h.bootTimer.Stop()
	}
	h.bootTimer = time.AfterFunc(bootBudget, func() { h.onBootTimeout(gen) })
	h.stateMu.Unlock()
}

// onBootTimeout is the backstop for a worker that never sends "ready" at
// all (onerror covers the common failure modes; this covers the rest, e.g.
// a fetch that hangs). Any pending request falls back to the main thread
// rather than waiting forever.
func (h *authoringHandle) onBootTimeout(gen uint64) {
	h.stateMu.Lock()
	if h.disposed || h.workerGen != gen || h.workerReady {
		h.stateMu.Unlock()
		return
	}
	h.workerBroken = true
	pending := h.pendingReq
	h.pendingReq = nil
	h.stateMu.Unlock()
	h.text("#ag-status", "Background compiler did not start in time; falling back to the main thread.")
	if pending != nil {
		go h.runLocalFallback(pending.seq, pending.grammarJSON, pending.source, pending.includeAnonymous)
	}
}

func (h *authoringHandle) handleWorkerMessage(gen uint64, args []js.Value) {
	if len(args) < 1 {
		return
	}
	data := args[0].Get("data")
	if !data.Truthy() {
		return
	}

	h.stateMu.Lock()
	if h.disposed || h.workerGen != gen {
		h.stateMu.Unlock()
		return
	}
	h.stateMu.Unlock()

	if jsString(data, "op") == "export" {
		h.handleExportResponse(data)
		return
	}

	if data.Get("ready").Truthy() {
		h.stateMu.Lock()
		h.workerReady = true
		if h.bootTimer != nil {
			h.bootTimer.Stop()
			h.bootTimer = nil
		}
		pending := h.pendingReq
		h.pendingReq = nil
		h.stateMu.Unlock()
		if pending != nil {
			h.postToWorker(pending)
		} else {
			h.text("#ag-status", "Background compiler ready…")
		}
		return
	}

	seqFloat := jsFloat(data, "seq")
	if seqFloat < 0 {
		// The worker bootstrap itself failed (see build-authoring-wasm's
		// generated authoring-worker.js catch handlers) before it ever had a
		// real request seq to reply to.
		h.stateMu.Lock()
		h.workerBroken = true
		h.stateMu.Unlock()
		message := jsString(data, "generateError")
		if message == "" {
			message = "background compiler failed to start"
		}
		h.text("#ag-status", message+"; falling back to the main thread.")
		return
	}
	seq := uint64(seqFloat)

	h.stateMu.Lock()
	stale := seq != h.runSeq
	if seq == h.inFlightSeq {
		h.inFlightSeq = 0
		h.disarmWatchdogLocked()
	}
	source := h.lastSource
	grammarJSON := h.lastGrammarJSON
	h.stateMu.Unlock()
	if stale {
		return
	}

	result, elapsedMs := decodeWorkerResult(data)
	h.render(grammarJSON, source, result, time.Duration(elapsedMs*float64(time.Millisecond)), "worker")
}

// armWatchdog (re)starts the soft/hard timers for seq, replacing whatever
// was tracking a previous (now superseded) request.
func (h *authoringHandle) armWatchdog(seq uint64) {
	h.stateMu.Lock()
	h.disarmWatchdogLocked()
	h.softTimer = time.AfterFunc(softBudget, func() { h.onSoftTimeout(seq) })
	h.hardTimer = time.AfterFunc(hardBudget, func() { h.onHardTimeout(seq) })
	h.stateMu.Unlock()
}

// disarmWatchdogLocked must be called with stateMu held.
func (h *authoringHandle) disarmWatchdogLocked() {
	if h.softTimer != nil {
		h.softTimer.Stop()
		h.softTimer = nil
	}
	if h.hardTimer != nil {
		h.hardTimer.Stop()
		h.hardTimer = nil
	}
}

func (h *authoringHandle) onSoftTimeout(seq uint64) {
	h.stateMu.Lock()
	active := !h.disposed && h.inFlightSeq == seq
	h.stateMu.Unlock()
	if !active {
		return
	}
	h.text("#ag-status", "Compiling a heavy grammar in the background worker… still making progress, this can take several seconds for a real-language base like Go.")
}

// onHardTimeout is the real safety net for risk #1: if the worker hasn't
// answered within hardBudget, terminate it (the only way to reclaim CPU from
// a Go/wasm goroutine that never reaches a cancellation checkpoint — see
// cmd/authoring-worker-wasm's generationBudget doc) and start a fresh one,
// then retry with whatever is currently in the editors.
func (h *authoringHandle) onHardTimeout(seq uint64) {
	h.stateMu.Lock()
	active := !h.disposed && h.inFlightSeq == seq
	if active {
		h.inFlightSeq = 0
	}
	h.stateMu.Unlock()
	if !active {
		return
	}
	h.text("#ag-status", "Grammar generation exceeded the time budget — restarting the background compiler. Try simplifying the grammar.")
	h.respawnWorker()
	h.run()
}

func (h *authoringHandle) respawnWorker() {
	h.stateMu.Lock()
	if h.disposed {
		h.stateMu.Unlock()
		return
	}
	old := h.worker
	oldFuncs := h.workerFuncs
	h.workerGen++
	gen := h.workerGen
	h.worker = js.Undefined()
	h.workerFuncs = nil
	h.stateMu.Unlock()

	for _, fn := range oldFuncs {
		fn.Release()
	}
	if old.Truthy() {
		old.Call("terminate")
	}
	h.spawnWorker(gen)
}

func (h *authoringHandle) fail(message string) {
	h.text("#ag-status", message)
	errors := h.find("#ag-errors")
	errors.Set("textContent", "")
	box := element("div")
	box.Set("className", "ag-err mono")
	box.Set("role", "alert")
	box.Set("textContent", message)
	errors.Call("appendChild", box)
}

// render draws one compile result. via records which code path produced it
// ("worker" or "main-thread") as a data attribute on the surface mount —
// cmd/verify-authoring-browser asserts on it to prove the seed grammar
// really did compile through the background Worker rather than silently
// falling back.
//
// grammarJSON is the exact (possibly base+delta merged) grammar.json that
// produced result — it is what onExportClick later sends back to the
// worker/local fallback for a Phase 3 export. render is where export
// readiness (canExport/exportGrammarJSON/exportName, and the button
// disabled state that reflects them) is decided: a compile only counts as
// "exportable" once generation itself succeeded — see
// CompileWithContextArtifacts's doc for why ParseError alone does not
// disqualify it (an author's grammar can be valid while their sample source
// has a typo) but ImportError/GenerateError/TimedOut do.
func (h *authoringHandle) render(grammarJSON, source string, result authoringengine.Result, elapsed time.Duration, via string) {
	if via != "" {
		h.mount.Get("dataset").Set("compileVia", via)
	}
	h.text("#ag-node-count", fmt.Sprintf("%d nodes", result.NodeCount))

	exportable := grammarJSON != "" && result.ImportError == "" && result.GenerateError == "" && !result.TimedOut
	h.stateMu.Lock()
	h.canExport = exportable
	if exportable {
		h.exportGrammarJSON = grammarJSON
		h.exportName = result.GrammarName
	}
	h.stateMu.Unlock()
	h.updateExportButtons(exportable)

	tree := h.find("#ag-tree")
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
		empty.Set("className", "ag-tree-empty")
		empty.Set("textContent", "No tree rows to display.")
		tree.Call("appendChild", empty)
	}

	errors := h.find("#ag-errors")
	errors.Set("textContent", "")
	for _, message := range []string{result.ImportError, result.GenerateError, result.ParseError} {
		if message == "" {
			continue
		}
		box := element("div")
		box.Set("className", "ag-err mono")
		box.Set("role", "alert")
		box.Set("textContent", message)
		errors.Call("appendChild", box)
	}

	h.renderConflicts(result.Conflicts, result.ConflictTotal, result.Warnings)
	h.renderHighlight(source, result.Highlights, result.HighlightNotice)

	switch {
	case result.ImportError != "":
		h.text("#ag-status", "grammar.json failed to import; see errors below.")
	case result.TimedOut:
		h.text("#ag-status", "Grammar generation timed out; see errors below. Try simplifying the grammar.")
	case result.GenerateError != "":
		h.text("#ag-status", "Grammar failed to compile; see errors below.")
	case result.ParseError != "":
		h.text("#ag-status", fmt.Sprintf("Compiled %q; sample failed to parse; see errors below.", result.GrammarName))
	case result.HasErrors:
		h.text("#ag-status", fmt.Sprintf("Compiled %q; sample has parse errors (%s).", result.GrammarName, elapsed.Round(100*time.Microsecond)))
	default:
		h.text("#ag-status", fmt.Sprintf("Compiled %q and parsed in %s (%d conflict%s).", result.GrammarName, elapsed.Round(100*time.Microsecond), result.ConflictTotal, plural(result.ConflictTotal)))
	}
}

func (h *authoringHandle) renderConflicts(conflicts []authoringengine.ConflictRow, total int, warnings []string) {
	if summary := h.find("#ag-conflict-summary"); summary.Truthy() {
		if total == 0 {
			summary.Set("textContent", "0 conflicts — this grammar is unambiguous as written.")
			summary.Set("className", "ag-status ag-status-ok")
		} else {
			summary.Set("textContent", fmt.Sprintf(
				"%d conflict%s encountered while building the LR table (grammargen auto-resolves each one via precedence/associativity/GLR — see detail below).",
				total, plural(total),
			))
			summary.Set("className", "ag-status")
		}
	}

	list := h.find("#ag-conflicts")
	if list.Truthy() {
		list.Set("textContent", "")
		const maxRendered = 200
		shown := conflicts
		truncated := total - len(conflicts)
		if len(shown) > maxRendered {
			truncated += len(shown) - maxRendered
			shown = shown[:maxRendered]
		}
		for _, c := range shown {
			row := element("div")
			row.Set("className", "ag-conflict")
			head := element("div")
			head.Set("className", "ag-conflict-head")
			head.Call("appendChild", styledText("span", "ag-conflict-kind", c.Kind))
			head.Call("appendChild", styledText("span", "ag-conflict-loc", fmt.Sprintf("state %d on %s", c.State, c.Lookahead)))
			row.Call("appendChild", head)
			row.Call("appendChild", styledText("p", "ag-conflict-res", c.Resolution))
			detail := element("pre")
			detail.Set("className", "ag-conflict-detail mono")
			detail.Set("textContent", c.Description)
			row.Call("appendChild", detail)
			list.Call("appendChild", row)
		}
		if truncated > 0 {
			note := element("p")
			note.Set("className", "ag-tree-empty")
			note.Set("textContent", fmt.Sprintf("…and %d more conflict%s (truncated).", truncated, plural(truncated)))
			list.Call("appendChild", note)
		}
	}

	warningBox := h.find("#ag-warnings")
	if warningBox.Truthy() {
		warningBox.Set("textContent", "")
		for _, w := range warnings {
			item := element("div")
			item.Set("className", "ag-warn mono")
			item.Set("textContent", w)
			warningBox.Call("appendChild", item)
		}
	}
}

func (h *authoringHandle) renderHighlight(source string, highlights []authoringengine.HighlightSpan, notice string) {
	container := h.find("#ag-highlight")
	if !container.Truthy() {
		return
	}
	container.Set("textContent", "")

	if len(highlights) == 0 {
		empty := element("p")
		empty.Set("className", "ag-tree-empty")
		if notice != "" {
			empty.Set("textContent", notice)
		} else {
			empty.Set("textContent", "No highlight spans.")
		}
		container.Call("appendChild", empty)
		return
	}

	pre := element("pre")
	pre.Set("className", "ag-hlpre mono")
	src := []byte(source)
	cursor := 0
	for _, span := range highlights {
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
			pre.Call("appendChild", textNode(string(src[cursor:start])))
		}
		styled := element("span")
		styled.Set("className", "ag-hl "+captureClass(span.Capture))
		styled.Call("setAttribute", "title", span.Capture)
		styled.Call("appendChild", textNode(string(src[start:end])))
		pre.Call("appendChild", styled)
		cursor = end
	}
	if cursor < len(src) {
		pre.Call("appendChild", textNode(string(src[cursor:])))
	}
	container.Call("appendChild", pre)
}

// captureClass buckets a capture name (e.g. "variable.parameter") by its
// top-level family ("variable") into one of eight deterministic palette
// classes defined in authoring.css.
func captureClass(capture string) string {
	family := capture
	if idx := strings.IndexByte(capture, '.'); idx >= 0 {
		family = capture[:idx]
	}
	hash := 0
	for _, r := range family {
		hash = hash*31 + int(r)
	}
	if hash < 0 {
		hash = -hash
	}
	return fmt.Sprintf("ag-hl-%d", hash%8)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// --- Phase 3: export ---
//
// The three download buttons (#ag-export-go/#ag-export-c/#ag-export-json)
// export the most recently SUCCESSFULLY compiled merged grammar — see
// render's export-readiness bookkeeping (canExport/exportGrammarJSON/
// exportName) — as one of grammargen's three pure export formats
// (authoringengine.ExportGrammar), reusing the worker's cached compile
// whenever possible (see cmd/authoring-worker-wasm's package doc) so a
// heavy base like Go does not re-pay LR-table generation just to download a
// file.

// bindExportButtons wires the three export buttons to onExportClick.
// Mirrors mountAuthoring's other bindings: a missing element is a mount-time
// error, since the page markup and this engine must agree on ids.
func (h *authoringHandle) bindExportButtons() error {
	for _, spec := range []struct {
		selector string
		format   string
	}{
		{"#ag-export-go", "go"},
		{"#ag-export-c", "c"},
		{"#ag-export-json", "json"},
	} {
		target := h.find(spec.selector)
		if !target.Truthy() {
			return fmt.Errorf("authoring element %s is missing", spec.selector)
		}
		format := spec.format
		fn := js.FuncOf(func(js.Value, []js.Value) any {
			h.onExportClick(format)
			return nil
		})
		target.Call("addEventListener", "click", fn)
		h.listeners = append(h.listeners, browserListener{target: target, event: "click", fn: fn})
	}
	return nil
}

// updateExportButtons reflects overall export readiness (enabled: has a
// successful compile happened, and did the LATEST one succeed) across all
// three export buttons, while leaving any individually busy button (its own
// in-flight export request — see setExportBusy) disabled regardless.
func (h *authoringHandle) updateExportButtons(enabled bool) {
	for _, id := range []string{"#ag-export-go", "#ag-export-c", "#ag-export-json"} {
		btn := h.find(id)
		if !btn.Truthy() {
			continue
		}
		format := strings.TrimPrefix(id, "#ag-export-")
		h.stateMu.Lock()
		busy := h.exportBusy[format]
		h.stateMu.Unlock()
		switch {
		case busy:
			btn.Set("disabled", true)
			btn.Set("title", "Generating…")
		case enabled:
			btn.Call("removeAttribute", "disabled")
			btn.Set("title", "")
		default:
			btn.Set("disabled", true)
			btn.Set("title", "Compile a grammar first")
		}
	}
}

// setExportBusy toggles one button's own in-flight state (independent of
// overall canExport) so a double-click on the same button cannot fire two
// overlapping export requests, then re-renders every button's disabled
// state (a busy button's own overlay always wins regardless of enabled).
func (h *authoringHandle) setExportBusy(format string, busy bool) {
	h.stateMu.Lock()
	if busy {
		h.exportBusy[format] = true
	} else {
		delete(h.exportBusy, format)
	}
	canExport := h.canExport
	h.stateMu.Unlock()
	h.updateExportButtons(canExport)
}

// onExportClick handles a click on one of the three export buttons. It
// exports h.exportGrammarJSON/h.exportName — the grammar from the most
// recent SUCCESSFUL compile (see render), not necessarily whatever is
// currently unsaved in the editors: the button is disabled whenever the
// latest compile failed, so by the time a click can land, exportGrammarJSON
// is known-good.
//
// The filename comes from whatever is currently typed in #ag-name (even if
// not yet recompiled — renaming is a purely cosmetic, instant thing an
// author would expect reflected in a download filename right away) and
// falls back to exportName (the grammar's own compiled name, itself already
// reflecting any previously-applied rename) when #ag-name is empty.
//
// If the background Worker is not currently ready, this exports via the
// same main-thread fallback compile path runLocalFallback uses for ordinary
// compiles, rather than queuing and waiting for the worker to finish
// booting — an export is a one-off user action, not a debounced stream of
// edits, so immediate best-effort service is preferable to an indefinite
// wait.
func (h *authoringHandle) onExportClick(format string) {
	h.stateMu.Lock()
	ready := h.canExport
	grammarJSON := h.exportGrammarJSON
	name := h.exportName
	busy := h.exportBusy[format]
	h.exportSeq++
	seq := h.exportSeq
	useWorker := !h.workerBroken && h.worker.Truthy() && h.workerReady
	worker := h.worker
	h.stateMu.Unlock()

	if busy {
		return
	}
	if !ready || grammarJSON == "" {
		h.text("#ag-export-status", "Compile a grammar first.")
		return
	}
	if el := h.find("#ag-name"); el.Truthy() {
		if override := strings.TrimSpace(el.Get("value").String()); override != "" {
			name = override
		}
	}

	h.setExportBusy(format, true)
	h.text("#ag-export-status", fmt.Sprintf("Generating %s…", exportLabel(format)))

	if !useWorker {
		go h.runExportLocalFallback(format, grammarJSON, name)
		return
	}
	worker.Call("postMessage", js.ValueOf(map[string]any{
		"op":          "export",
		"seq":         float64(seq),
		"format":      format,
		"grammarJSON": grammarJSON,
		"name":        name,
		"pkg":         "",
	}))
}

// runExportLocalFallback mirrors runLocalFallback for exports: used only
// when the background Worker is unavailable, so — unlike the worker's own
// cache (see cmd/authoring-worker-wasm's package doc) — there is nothing to
// reuse here; it always compiles grammarJSON fresh via
// authoringengine.ImportAndGenerateWithContext. This is an accepted
// limitation of the already-degraded no-Worker path, not a Phase 3 gap: a
// browser without Worker support (or a crashed worker) already pays the
// full compile cost on every edit, not just on export.
func (h *authoringHandle) runExportLocalFallback(format, grammarJSON, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	grammar, lang, err := authoringengine.ImportAndGenerateWithContext(ctx, grammarJSON)
	if err != nil {
		h.finishExport(format, "", "", err.Error())
		return
	}
	filename, content, err := authoringengine.ExportGrammar(grammar, lang, authoringengine.ExportFormat(format), name, "")
	if err != nil {
		h.finishExport(format, "", "", err.Error())
		return
	}
	h.finishExport(format, filename, content, "")
}

// handleExportResponse handles one {op:"export", ...} message from the
// worker — see cmd/authoring-worker-wasm's package doc for the wire shape.
// The response's own seq is not used for staleness gating the way a compile
// response's is (see the authoringHandle.exportSeq doc): every export
// request is an independent, self-contained user action whose result is
// always honored whenever it arrives, not superseded by a later one.
func (h *authoringHandle) handleExportResponse(data js.Value) {
	format := jsString(data, "format")
	errMsg := jsString(data, "error")
	filename := jsString(data, "filename")
	content := jsString(data, "content")
	h.finishExport(format, filename, content, errMsg)
}

// finishExport is the single landing point for every export outcome
// (worker response or local fallback): it clears the button's busy state
// and either triggers the browser download or reports the failure.
func (h *authoringHandle) finishExport(format, filename, content, errMsg string) {
	h.setExportBusy(format, false)
	if errMsg != "" {
		h.text("#ag-export-status", fmt.Sprintf("%s export failed: %s", exportLabel(format), errMsg))
		return
	}
	triggerDownload(filename, content)
	h.text("#ag-export-status", fmt.Sprintf("Downloaded %s", filename))
}

// triggerDownload performs a client-side Blob + object URL + <a download>
// click — the standard way to save generated content to disk without a
// server round trip. filename/content never touch the network; the only
// thing that leaves this tab is whatever the browser's own download
// mechanism writes to the user's own disk.
func triggerDownload(filename, content string) {
	blobParts := js.Global().Get("Array").New(1)
	blobParts.SetIndex(0, content)
	blob := js.Global().Get("Blob").New(blobParts, js.ValueOf(map[string]any{"type": "application/octet-stream"}))
	url := js.Global().Get("URL").Call("createObjectURL", blob)

	a := element("a")
	a.Set("href", url)
	a.Set("download", filename)
	a.Get("style").Set("display", "none")
	doc := js.Global().Get("document")
	doc.Get("body").Call("appendChild", a)
	a.Call("click")
	doc.Get("body").Call("removeChild", a)
	js.Global().Get("URL").Call("revokeObjectURL", url)
}

func exportLabel(format string) string {
	switch format {
	case "go":
		return ".go"
	case "c":
		return "parser.c"
	case "json":
		return "grammar.json"
	default:
		return format
	}
}

// decodeWorkerResult reconstructs an authoringengine.Result from a worker
// response message — see cmd/authoring-worker-wasm's package doc for the
// wire schema both sides agree on.
func decodeWorkerResult(data js.Value) (result authoringengine.Result, elapsedMs float64) {
	result.GrammarName = jsString(data, "grammarName")
	result.ImportError = jsString(data, "importError")
	result.GenerateError = jsString(data, "generateError")
	result.ParseError = jsString(data, "parseError")
	result.TimedOut = jsBool(data, "timedOut")
	result.HasErrors = jsBool(data, "hasErrors")
	result.NodeCount = jsInt(data, "nodeCount")
	result.ConflictTotal = jsInt(data, "conflictTotal")
	result.HighlightNotice = jsString(data, "highlightNotice")
	elapsedMs = jsFloat(data, "elapsedMs")

	if rows := data.Get("treeRows"); rows.Truthy() {
		n := rows.Length()
		result.TreeRows = make([]authoringengine.TreeRow, 0, n)
		for i := 0; i < n; i++ {
			item := rows.Index(i)
			result.TreeRows = append(result.TreeRows, authoringengine.TreeRow{
				Class:   jsString(item, "class"),
				Depth:   jsString(item, "depth"),
				Level:   jsInt(item, "level"),
				Field:   jsString(item, "field"),
				Type:    jsString(item, "type"),
				Range:   jsString(item, "range"),
				Missing: jsBool(item, "missing"),
			})
		}
	}
	if items := data.Get("conflicts"); items.Truthy() {
		n := items.Length()
		result.Conflicts = make([]authoringengine.ConflictRow, 0, n)
		for i := 0; i < n; i++ {
			item := items.Index(i)
			result.Conflicts = append(result.Conflicts, authoringengine.ConflictRow{
				Kind:        jsString(item, "kind"),
				State:       jsInt(item, "state"),
				Lookahead:   jsString(item, "lookahead"),
				Resolution:  jsString(item, "resolution"),
				Description: jsString(item, "description"),
			})
		}
	}
	if items := data.Get("warnings"); items.Truthy() {
		n := items.Length()
		result.Warnings = make([]string, 0, n)
		for i := 0; i < n; i++ {
			result.Warnings = append(result.Warnings, items.Index(i).String())
		}
	}
	if items := data.Get("highlights"); items.Truthy() {
		n := items.Length()
		result.Highlights = make([]authoringengine.HighlightSpan, 0, n)
		for i := 0; i < n; i++ {
			item := items.Index(i)
			result.Highlights = append(result.Highlights, authoringengine.HighlightSpan{
				StartByte: jsInt(item, "startByte"),
				EndByte:   jsInt(item, "endByte"),
				Capture:   jsString(item, "capture"),
			})
		}
	}
	return result, elapsedMs
}

func jsString(v js.Value, key string) string {
	field := v.Get(key)
	if field.Type() != js.TypeString {
		return ""
	}
	return field.String()
}

func jsBool(v js.Value, key string) bool {
	field := v.Get(key)
	return field.Type() == js.TypeBoolean && field.Bool()
}

func jsInt(v js.Value, key string) int {
	field := v.Get(key)
	if field.Type() != js.TypeNumber {
		return 0
	}
	return field.Int()
}

func jsFloat(v js.Value, key string) float64 {
	field := v.Get(key)
	if field.Type() != js.TypeNumber {
		return 0
	}
	return field.Float()
}

func (h *authoringHandle) find(selector string) js.Value {
	return h.mount.Call("querySelector", selector)
}

func (h *authoringHandle) text(selector, value string) {
	target := h.find(selector)
	if target.Truthy() {
		target.Set("textContent", value)
	}
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

// --- fetch helpers ---
//
// cmd/authoring-wasm has no bootstrap fetch of its own through Phase 1
// (grammargen compiles the hand-typed textarea live; nothing is loaded over
// the network). Phase 2's base picker needs one — same shape as
// cmd/playground-wasm's fetchJSON/fetchBytes/await (duplicated here rather
// than factored into a shared package: each wasm command in this repo is its
// own small `package main`, and these four functions are a few lines of
// standard js.Value plumbing, not application logic worth a shared
// dependency).

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
