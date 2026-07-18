//go:build js && wasm

package main

import (
	"context"
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
	// Sized off a measured worst case, not a guess: authoringengine.diagnose
	// runs a second full LR generation pass (no cancellable+diagnostics
	// combo exists in the pinned grammargen release — see its doc), so a
	// genuinely heavy grammar pays for generation twice. Natively,
	// grammargen.LoxGrammar() — the design doc's canonical "expensive"
	// grammar — costs ~824ms+864ms (~1.7s combined); Go/wasm typically runs
	// 2-4x slower, so a legitimate (not pathological) heavy grammar can
	// reasonably take several seconds. softBudget should fire well before
	// that to reassure the author it hasn't hung; hardBudget must stay well
	// above it so LoxGrammar-scale grammars don't get spuriously killed.
	softBudget = 4 * time.Second
	hardBudget = 15 * time.Second
	// bootBudget bounds how long a newly spawned worker has to send its
	// "ready" ping (importScripts + fetch + WebAssembly.instantiate +
	// go.run for a ~17MB binary) before a pending request falls back to the
	// main thread. onerror already covers most startup failures; this is
	// the backstop for the rest (e.g. a fetch that hangs without erroring).
	bootBudget = 20 * time.Second
)

type authoringProps struct {
	WorkerScriptURL string `json:"workerScriptURL"`
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
	// response (0 = idle). lastSource is the sample text sent alongside it,
	// snapshotted so the highlight renderer uses exactly the bytes the
	// worker computed byte offsets against, even if the textarea has since
	// been edited again (that edit would have already produced a newer,
	// non-matching runSeq, so this response would be dropped as stale
	// before lastSource is ever read for it).
	inFlightSeq uint64
	lastSource  string
	softTimer   *time.Timer
	hardTimer   *time.Timer
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

// mountAuthoring wires the grammar-authoring surface: a grammar.json
// textarea (#ag-grammar), a sample-source textarea (#ag-source), an
// anonymous-node toggle (#ag-anonymous), and the render sinks
// (#ag-tree/#ag-errors/#ag-status/#ag-node-count/#ag-conflicts/
// #ag-warnings/#ag-highlight). It needs no bootstrap fetch of its own —
// grammargen compiles the grammar live from the textarea — but it does spawn
// the background compiler Worker so the very first (seed) compile already
// goes through it.
func mountAuthoring(ctx enginewasm.Context) (enginewasm.Handle, error) {
	mount := ctx.Mount()
	if !mount.Truthy() {
		return nil, fmt.Errorf("authoring surface requires a DOM mount")
	}
	h := &authoringHandle{mount: mount}
	var props authoringProps
	if err := ctx.DecodeProps(&props); err == nil {
		h.workerScriptURL = strings.TrimSpace(props.WorkerScriptURL)
	}

	for _, binding := range []struct {
		selector  string
		event     string
		immediate bool
	}{
		{"#ag-grammar", "input", false},
		{"#ag-source", "input", false},
		{"#ag-anonymous", "change", true},
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
	h.mount.Get("dataset").Set("privacyBoundary", "browser-only")

	if h.workerScriptURL == "" {
		h.workerBroken = true
	} else {
		h.spawnWorker(0)
	}
	h.run()
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

	grammarJSON := h.find("#ag-grammar").Get("value").String()
	source := h.find("#ag-source").Get("value").String()
	includeAnonymous := true
	if el := h.find("#ag-anonymous"); el.Truthy() {
		includeAnonymous = el.Get("checked").Bool()
	}
	h.dispatch(seq, grammarJSON, source, includeAnonymous)
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
		h.render(source, result, time.Since(started), "main-thread")
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
	h.stateMu.Unlock()
	if stale {
		return
	}

	result, elapsedMs := decodeWorkerResult(data)
	h.render(source, result, time.Duration(elapsedMs*float64(time.Millisecond)), "worker")
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
	h.text("#ag-status", "Still compiling in the background worker… this grammar may be complex.")
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
func (h *authoringHandle) render(source string, result authoringengine.Result, elapsed time.Duration, via string) {
	if via != "" {
		h.mount.Get("dataset").Set("compileVia", via)
	}
	h.text("#ag-node-count", fmt.Sprintf("%d nodes", result.NodeCount))

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
