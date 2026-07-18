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
//	compile request   {seq, grammarJSON, source, includeAnonymous}
//	compile response  {seq, grammarName, importError, generateError, parseError,
//	                   timedOut, hasErrors, nodeCount, elapsedMs,
//	                   treeRows:[{class,depth,level,field,type,range,missing}],
//	                   conflicts:[{kind,state,lookahead,resolution,description}],
//	                   conflictTotal, warnings:[string],
//	                   highlights:[{startByte,endByte,capture}], highlightNotice}
//
//	export request     {op:"export", seq, format:"go"|"c"|"json", grammarJSON, name, pkg}
//	export response    {op:"export", seq, format, filename, content, error}
//
// A message with op=="export" is Phase 3's addition; every other message
// (op absent, the shape every Phase 1/2 caller already sends) is a compile
// request, unchanged. The main thread tags every compile request with a
// monotonically increasing seq and drops any compile response whose seq is
// not its latest — this program does not need to know that; it just answers
// requests in the order it receives them and lets the caller decide what's
// stale. Export requests are independent, user-button-triggered actions
// (not superseded by a newer one the way an edit supersedes an in-flight
// compile), so they are not seq-gated on this side either — see
// cmd/authoring-wasm's onExportClick.
//
// Export caching (the "do NOT recompile from scratch on export" requirement):
// every successful compile response also updates a package-level cache of
// the exact (grammarJSON, *grammargen.Grammar, *gts.Language) triple that
// compile produced (see updateCache). An export request whose grammarJSON
// matches the cache reuses it directly — for a heavy base like Go this is
// the difference between an export completing near-instantly and re-paying
// the full LR-table generation cost. Only a genuine cache miss (the author
// edited the grammar and clicked export before the next debounced compile
// landed, or no compile has completed yet) falls back to compiling on
// demand via authoringengine.ImportAndGenerateWithContext.
package main

import (
	"context"
	"sync"
	"syscall/js"
	"time"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter-docs/internal/authoringengine"
	"github.com/odvcencio/gotreesitter/grammargen"
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

// cache holds the (grammarJSON, *grammargen.Grammar, *gts.Language) triple
// from the most recent successful compile — see the package doc's "Export
// caching" section. cacheSeq guards against a superseded compile goroutine
// finishing late and clobbering a newer cache entry with stale data: it is
// monotonic, so an update only takes effect if its seq is not older than
// whatever is already cached.
var (
	cacheMu          sync.Mutex
	cacheSeq         float64
	cacheGrammarJSON string
	cacheGrammar     *grammargen.Grammar
	cacheLanguage    *gts.Language
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

	if jsString(data, "op") == "export" {
		// Independent of the compile pipeline below: does not touch
		// inflightCancel (an export must not cancel an in-flight compile,
		// nor vice versa) and runs in its own goroutine so a cache-miss
		// on-demand compile-for-export can't block a concurrent compile
		// request's response.
		go respondExport(data)
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
	result, grammar, lang := authoringengine.CompileWithContextArtifacts(ctx, grammarJSON, source, includeAnonymous)
	if grammar != nil && lang != nil {
		updateCache(seq, grammarJSON, grammar, lang)
	}
	elapsedMs := float64(time.Since(started)) / float64(time.Millisecond)
	js.Global().Call("postMessage", encodeResult(seq, elapsedMs, result))
}

// updateCache records the (grammarJSON, grammar, lang) triple from a
// successful compile, unless a newer compile (higher seq) already holds the
// cache — see the package doc's "Export caching" section.
func updateCache(seq float64, grammarJSON string, grammar *grammargen.Grammar, lang *gts.Language) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if seq < cacheSeq {
		return
	}
	cacheSeq = seq
	cacheGrammarJSON = grammarJSON
	cacheGrammar = grammar
	cacheLanguage = lang
}

// cachedArtifacts returns the cached (grammar, lang) pair only if it was
// produced by compiling exactly grammarJSON — i.e. the cache is still
// current for what the author has authored right now. Any mismatch (nothing
// cached yet, or the author edited since the last successful compile) is
// treated as a cache miss so respondExport falls back to compiling on
// demand rather than exporting a different grammar than the one requested.
func cachedArtifacts(grammarJSON string) (*grammargen.Grammar, *gts.Language) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cacheGrammar == nil || cacheLanguage == nil || cacheGrammarJSON != grammarJSON {
		return nil, nil
	}
	return cacheGrammar, cacheLanguage
}

// respondExport handles one {op:"export", ...} request — see the package
// doc's wire protocol and "Export caching" sections.
func respondExport(data js.Value) {
	seq := data.Get("seq").Float()
	format := jsString(data, "format")
	grammarJSON := jsString(data, "grammarJSON")
	name := jsString(data, "name")
	pkg := jsString(data, "pkg")

	grammar, lang := cachedArtifacts(grammarJSON)
	if grammar == nil {
		// Cache miss: compile on demand. This is the one export path that
		// can re-pay LR-table generation — expected only when the author
		// clicked export before the debounced compile of their latest edit
		// landed, or before any compile has completed at all.
		ctx, cancel := context.WithTimeout(context.Background(), generationBudget)
		compiled, compiledLang, err := authoringengine.ImportAndGenerateWithContext(ctx, grammarJSON)
		cancel()
		if err != nil {
			js.Global().Call("postMessage", encodeExportError(seq, format, err.Error()))
			return
		}
		grammar, lang = compiled, compiledLang
		updateCache(seq, grammarJSON, grammar, lang)
	}

	filename, content, err := authoringengine.ExportGrammar(grammar, lang, authoringengine.ExportFormat(format), name, pkg)
	if err != nil {
		js.Global().Call("postMessage", encodeExportError(seq, format, err.Error()))
		return
	}
	js.Global().Call("postMessage", js.ValueOf(map[string]any{
		"op":       "export",
		"seq":      seq,
		"format":   format,
		"filename": filename,
		"content":  content,
		"error":    "",
	}))
}

func encodeExportError(seq float64, format, message string) js.Value {
	return js.ValueOf(map[string]any{
		"op":       "export",
		"seq":      seq,
		"format":   format,
		"filename": "",
		"content":  "",
		"error":    message,
	})
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
