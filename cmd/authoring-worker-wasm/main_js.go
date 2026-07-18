//go:build js && wasm

// Command authoring-worker-wasm is the Phase 1 grammar-authoring compiler,
// relocated off the main thread. It is a plain (non-GoSX) js/wasm build —
// the same shape as the engine repo's wasm/grammargen bridge — because
// GoSX's engine/wasm island runtime (cmd/authoring-wasm) is a DOM-mounting
// abstraction with no meaning inside a Web Worker's global scope. The main
// thread keeps owning the GoSX island (props, DOM, listeners); this binary
// owns nothing but a self.onmessage handler and the CPU-heavy
// import->generate->diagnose->highlight->parse loop.
//
// Wire protocol (structured-clone JS objects over postMessage):
//
//	request  {seq, grammarJSON, source, includeAnonymous}
//	response {seq, grammarName, importError, generateError, parseError,
//	          timedOut, hasErrors, nodeCount, elapsedMs,
//	          treeRows:[{class,depth,level,field,type,range,missing}],
//	          conflicts:[{kind,state,lookahead,resolution,description}],
//	          conflictTotal, warnings:[string],
//	          highlights:[{startByte,endByte,capture}], highlightNotice}
//
// The main thread tags every request with a monotonically increasing seq
// and drops any response whose seq is not its latest — this program does
// not need to know that; it just answers requests in the order it receives
// them and lets the caller decide what's stale.
package main

import (
	"context"
	"sync"
	"syscall/js"
	"time"

	"github.com/odvcencio/gotreesitter-docs/internal/authoringengine"
)

// generationBudget bounds grammargen.GenerateLanguageWithContext. It is
// checked at grammargen's internal phase/periodic checkpoints — see the
// package doc on CompileWithContext and cmd/build-authoring-wasm's README
// comment for the honest caveat: a Go/wasm goroutine in a tight CPU loop
// cannot be preempted mid-flight on this single-threaded target, so this
// budget only fires at a checkpoint the running generation actually
// reaches. The main thread runs its own watchdog (terminate + respawn,
// hardBudget in cmd/authoring-wasm) as the real backstop for a generation
// that never reaches one — this constant is deliberately below that
// main-thread hardBudget so a grammar that legitimately exceeds it (rather
// than hanging outright) gets this package's own "timed out" diagnostic
// instead of a bare worker termination.
const generationBudget = 12 * time.Second

var (
	inflightMu     sync.Mutex
	inflightCancel context.CancelFunc
)

func main() {
	self := js.Global()
	self.Set("onmessage", js.FuncOf(handleMessage))
	self.Call("postMessage", js.ValueOf(map[string]any{"ready": true}))
	select {}
}

func handleMessage(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return nil
	}
	data := args[0].Get("data")
	if !data.Truthy() {
		return nil
	}
	seq := data.Get("seq").Float()
	grammarJSON := jsString(data, "grammarJSON")
	source := jsString(data, "source")
	includeAnonymous := true
	if v := data.Get("includeAnonymous"); v.Type() == js.TypeBoolean {
		includeAnonymous = v.Bool()
	}

	inflightMu.Lock()
	if inflightCancel != nil {
		// A newer request supersedes whatever is still in flight. This is
		// best-effort: it only takes effect if the superseded goroutine
		// hasn't already run past its last ctx.Err() checkpoint inside
		// grammargen (see generationBudget doc above).
		inflightCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), generationBudget)
	inflightCancel = cancel
	inflightMu.Unlock()

	go respond(ctx, cancel, seq, grammarJSON, source, includeAnonymous)
	return nil
}

func respond(ctx context.Context, cancel context.CancelFunc, seq float64, grammarJSON, source string, includeAnonymous bool) {
	defer cancel()
	started := time.Now()
	result := authoringengine.CompileWithContext(ctx, grammarJSON, source, includeAnonymous)
	elapsedMs := float64(time.Since(started)) / float64(time.Millisecond)
	js.Global().Call("postMessage", encodeResult(seq, elapsedMs, result))
}

func jsString(obj js.Value, key string) string {
	v := obj.Get(key)
	if v.Type() != js.TypeString {
		return ""
	}
	return v.String()
}

func encodeResult(seq, elapsedMs float64, result authoringengine.Result) js.Value {
	treeRows := make([]any, len(result.TreeRows))
	for i, row := range result.TreeRows {
		treeRows[i] = map[string]any{
			"class":   row.Class,
			"depth":   row.Depth,
			"level":   row.Level,
			"field":   row.Field,
			"type":    row.Type,
			"range":   row.Range,
			"missing": row.Missing,
		}
	}
	conflicts := make([]any, len(result.Conflicts))
	for i, c := range result.Conflicts {
		conflicts[i] = map[string]any{
			"kind":        c.Kind,
			"state":       c.State,
			"lookahead":   c.Lookahead,
			"resolution":  c.Resolution,
			"description": c.Description,
		}
	}
	warnings := make([]any, len(result.Warnings))
	for i, w := range result.Warnings {
		warnings[i] = w
	}
	highlights := make([]any, len(result.Highlights))
	for i, h := range result.Highlights {
		highlights[i] = map[string]any{
			"startByte": h.StartByte,
			"endByte":   h.EndByte,
			"capture":   h.Capture,
		}
	}
	return js.ValueOf(map[string]any{
		"seq":             seq,
		"grammarName":     result.GrammarName,
		"importError":     result.ImportError,
		"generateError":   result.GenerateError,
		"parseError":      result.ParseError,
		"timedOut":        result.TimedOut,
		"hasErrors":       result.HasErrors,
		"nodeCount":       result.NodeCount,
		"elapsedMs":       elapsedMs,
		"treeRows":        treeRows,
		"conflicts":       conflicts,
		"conflictTotal":   result.ConflictTotal,
		"warnings":        warnings,
		"highlights":      highlights,
		"highlightNotice": result.HighlightNotice,
	})
}
