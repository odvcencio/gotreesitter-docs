// Package authoringengine implements the grammar-authoring playground's
// author -> compile -> parse loop: import a tree-sitter grammar.json,
// compile it in-memory with grammargen, and parse a sample source against
// the freshly generated language. In production the only caller is the
// browser WebAssembly engine — Phase 0 called it directly from the
// main-thread GoSX island (cmd/authoring-wasm); Phase 1 moved the primary
// compile+parse work into a Web Worker (cmd/authoring-worker-wasm) so a
// heavy grammar cannot freeze the tab. cmd/authoring-wasm still imports this
// package too, but only as a main-thread fallback for browsers without Web
// Worker support (or if the worker crashes) — every normal request goes
// through the worker. The docs server never imports this package or
// receives editor contents.
package authoringengine

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

const (
	// MaxGrammarBytes caps the grammar.json textarea.
	MaxGrammarBytes = 64 << 10
	MaxSourceBytes  = 64 << 10
	MaxTreeRows     = 1200
	// MaxHighlightSpans caps the highlight overlay so a pathological grammar
	// or sample cannot balloon the response payload.
	MaxHighlightSpans = 2000
	// MaxConflicts caps how many ConflictRow entries diagnose() builds and
	// the worker ships over postMessage. A single heavy grammar (e.g.
	// grammargen.LoxGrammar(), the design's canonical "expensive" example)
	// can report 60k+ raw conflicts — grammargen.GenerateWithReport already
	// paid for constructing all of them internally (see diagnose's doc), but
	// formatting and marshalling that many ConflictDiag.String() renderings
	// is unbounded work this package controls. Result.ConflictTotal reports
	// the true count even when Conflicts is truncated.
	MaxConflicts = 500
)

type TreeRow struct {
	Class   string
	Depth   string
	Level   int
	Field   string
	Type    string
	Range   string
	Missing bool
}

// ConflictRow is a browser-friendly view of one grammargen.ConflictDiag:
// one LR-table state/lookahead pair where more than one action was
// available and grammargen had to pick a winner (or, for GLR, kept more
// than one). Kind/State/Lookahead/Resolution back small styled badges;
// Description is the multi-line grammargen.ConflictDiag.String(ng)
// rendering (production text + resolution) for anyone who wants detail.
type ConflictRow struct {
	Kind        string
	State       int
	Lookahead   string
	Resolution  string
	Description string
}

// HighlightSpan is one styled byte range from grammargen's
// auto-generated highlight query, run against the sample source.
type HighlightSpan struct {
	StartByte int
	EndByte   int
	Capture   string
}

// Result carries the outcome of one compile+parse pass. At most one of
// ImportError/GenerateError/ParseError is set for a given failing run.
// Conflicts/Warnings/Highlights are best-effort: a failure computing any of
// them never prevents the syntax tree (or the other diagnostics) from being
// reported.
type Result struct {
	TreeRows      []TreeRow
	NodeCount     int
	HasErrors     bool
	GrammarName   string
	ImportError   string
	GenerateError string
	ParseError    string

	Conflicts []ConflictRow
	// ConflictTotal is the true number of conflicts grammargen encountered,
	// even when len(Conflicts) was truncated to MaxConflicts.
	ConflictTotal int
	Warnings      []string

	Highlights      []HighlightSpan
	HighlightNotice string // e.g. "no highlight query generated for this grammar"

	// TimedOut is set when GenerateError was caused by ctx being cancelled
	// or its deadline exceeded — i.e. the grammar likely blew the
	// generation time budget rather than being structurally invalid.
	TimedOut bool
}

// Compile is CompileWithContext with a background (uncancellable,
// unbounded) context. Kept for callers — and tests — that don't need a
// generation budget.
func Compile(grammarJSON, source string, includeAnonymous bool) Result {
	return CompileWithContext(context.Background(), grammarJSON, source, includeAnonymous)
}

// CompileWithContext runs the full grammar.json -> Grammar -> Language ->
// Tree loop entirely inside its caller. It never touches the filesystem or
// network: grammargen.ImportGrammarJSON and grammargen.GenerateLanguageWithContext
// operate on in-memory bytes/structs only, so this is safe to call from the
// browser WebAssembly worker on every debounced edit.
//
// ctx bounds only the LR-table generation step
// (grammargen.GenerateLanguageWithContext) — the one exported, genuinely
// cancellable/timeout-aware entry point grammargen offers. The diagnostic
// report (conflicts/warnings) and the highlight query both re-run
// generation-shaped work without a context parameter; by the time control
// reaches them the primary generation has already completed within budget,
// so in practice they finish in comparable time. See cmd/authoring-worker-wasm
// for why this is a best-effort, not a hard guarantee, on a single-threaded
// Go/wasm target.
func CompileWithContext(ctx context.Context, grammarJSON, source string, includeAnonymous bool) (result Result) {
	result.TreeRows = []TreeRow{}
	defer func() {
		if recovered := recover(); recovered != nil {
			result.GenerateError = fmt.Sprintf("grammar compiler panicked: %v", recovered)
		}
	}()
	if ctx == nil {
		ctx = context.Background()
	}

	if len(grammarJSON) > MaxGrammarBytes {
		result.ImportError = "grammar.json must be 64 KiB or smaller"
		return result
	}
	if len(source) > MaxSourceBytes {
		result.ParseError = "sample source must be 64 KiB or smaller"
		return result
	}

	g, err := grammargen.ImportGrammarJSON([]byte(grammarJSON))
	if err != nil {
		result.ImportError = err.Error()
		return result
	}
	result.GrammarName = g.Name

	lang, err := grammargen.GenerateLanguageWithContext(ctx, g)
	if err != nil {
		if isContextErr(err) {
			result.TimedOut = true
			result.GenerateError = "grammar generation was cancelled or exceeded the time budget (the grammar may be too complex): " + err.Error()
		} else {
			result.GenerateError = err.Error()
		}
		return result
	}
	if lang == nil {
		result.GenerateError = "grammar compiled to a nil language"
		return result
	}

	result.Conflicts, result.ConflictTotal, result.Warnings = diagnose(g)
	result.Highlights, result.HighlightNotice = computeHighlights(g, lang, source)

	parser := gts.NewParser(lang)
	tree, err := parser.Parse([]byte(source))
	if err != nil {
		result.ParseError = err.Error()
		return result
	}
	if tree == nil || tree.RootNode() == nil {
		result.ParseError = "parser returned no syntax tree"
		return result
	}
	defer tree.Release()

	appendTreeRows(tree.RootNode(), lang, includeAnonymous, 0, "", &result)
	return result
}

// isContextErr reports whether err is (or wraps) a context cancellation or
// deadline error, as opposed to a genuine grammar problem.
func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// diagnose runs grammargen.GenerateWithReport purely for its Conflicts and
// Warnings — the compiled Language it also returns is discarded in favor of
// the one CompileWithContext already built via the cancellable
// GenerateLanguageWithContext path. This is a real second LR-table
// generation pass; grammargen's pinned release exposes no
// context/cancellable variant that also returns diagnostics, so a grammar
// that legitimately compiles (rather than timing out on the first,
// cancellable pass) pays for generation twice. Measured natively:
// grammargen.LoxGrammar(), the design doc's canonical "expensive" example,
// costs ~824ms for GenerateLanguageWithContext and ~864ms for
// GenerateWithReport — roughly double, not exponential — so this is a
// tolerable, documented tradeoff for Phase 1, not a hidden blowup. Formatting
// cost (ConflictDiag.String() per row) is separately bounded by MaxConflicts
// below, since raw conflict counts (not just wall time) can be extreme:
// LoxGrammar() alone reports 63,461 raw conflicts.
//
// Best-effort: any failure (including a panic deep in grammargen) is
// swallowed so a diagnostics bug never takes down tree rendering.
func diagnose(g *grammargen.Grammar) (rows []ConflictRow, total int, warnings []string) {
	defer func() { recover() }()

	report, err := grammargen.GenerateWithReport(g)
	if err != nil || report == nil {
		return nil, 0, nil
	}
	warnings = report.Warnings
	total = len(report.Conflicts)

	ng, ngErr := grammargen.Normalize(g)
	limit := total
	if limit > MaxConflicts {
		limit = MaxConflicts
	}
	rows = make([]ConflictRow, 0, limit)
	for i := 0; i < limit; i++ {
		diag := &report.Conflicts[i]
		row := ConflictRow{
			Kind:       conflictKindLabel(diag),
			State:      diag.State,
			Resolution: diag.Resolution,
		}
		if ngErr == nil && ng != nil {
			row.Lookahead = symbolName(ng, diag.LookaheadSym)
			row.Description = diag.String(ng)
		} else {
			row.Lookahead = fmt.Sprintf("sym_%d", diag.LookaheadSym)
			row.Description = fmt.Sprintf("%s conflict in state %d on %s: %s", row.Kind, row.State, row.Lookahead, row.Resolution)
		}
		rows = append(rows, row)
	}
	return rows, total, warnings
}

func conflictKindLabel(diag *grammargen.ConflictDiag) string {
	if diag.Kind == grammargen.ReduceReduce {
		return "reduce/reduce"
	}
	return "shift/reduce"
}

func symbolName(ng *grammargen.NormalizedGrammar, id int) string {
	if ng != nil && id >= 0 && id < len(ng.Symbols) {
		return ng.Symbols[id].Name
	}
	return fmt.Sprintf("sym_%d", id)
}

// computeHighlights auto-generates highlight queries for the whole grammar
// (via grammargen.GenerateHighlightQueries against an empty base — Phase 1
// authors a single grammar, not yet an inherited extension, so every rule
// counts as "new") and runs them against source. Best-effort: any failure
// leaves Highlights empty with a human HighlightNotice rather than failing
// the compile.
func computeHighlights(g *grammargen.Grammar, lang *gts.Language, source string) (spans []HighlightSpan, notice string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			spans = nil
			notice = fmt.Sprintf("highlight generation panicked: %v", recovered)
		}
	}()

	query := strings.TrimSpace(grammargen.GenerateHighlightQueries(&grammargen.Grammar{}, g))
	if query == "" {
		return nil, "grammargen found no keyword/operator/declaration-shaped rules to auto-highlight in this grammar"
	}

	highlighter, err := gts.NewHighlighter(lang, query)
	if err != nil {
		return nil, "highlight query failed to compile: " + err.Error()
	}

	ranges := highlighter.Highlight([]byte(source))
	if len(ranges) == 0 {
		return nil, "highlight query compiled but matched nothing in the sample"
	}
	spans = make([]HighlightSpan, 0, len(ranges))
	for _, r := range ranges {
		spans = append(spans, HighlightSpan{
			StartByte: int(r.StartByte),
			EndByte:   int(r.EndByte),
			Capture:   r.Capture,
		})
		if len(spans) >= MaxHighlightSpans {
			break
		}
	}
	return spans, ""
}

func appendTreeRows(node *gts.Node, lang *gts.Language, includeAnonymous bool, depth int, field string, result *Result) {
	if node == nil || len(result.TreeRows) >= MaxTreeRows {
		return
	}
	result.NodeCount++
	if node.IsError() || node.IsMissing() {
		result.HasErrors = true
	}
	if includeAnonymous || node.IsNamed() {
		className := "ag-tline"
		if !node.IsNamed() {
			className += " ag-anonrow"
		}
		if node.IsError() || node.IsMissing() {
			className += " ag-err-row"
		}
		result.TreeRows = append(result.TreeRows, TreeRow{
			Class:   className,
			Depth:   strings.Repeat("  ", depth),
			Level:   depth + 1,
			Field:   field,
			Type:    node.Type(lang),
			Range:   formatRange(node.StartPoint(), node.EndPoint()),
			Missing: node.IsMissing(),
		})
	}
	for i := 0; i < node.ChildCount(); i++ {
		appendTreeRows(node.Child(i), lang, includeAnonymous, depth+1, node.FieldNameForChild(i, lang), result)
		if len(result.TreeRows) >= MaxTreeRows {
			return
		}
	}
}

func formatRange(start, end gts.Point) string {
	return strconv.FormatUint(uint64(start.Row+1), 10) + ":" + strconv.FormatUint(uint64(start.Column+1), 10) + "–" +
		strconv.FormatUint(uint64(end.Row+1), 10) + ":" + strconv.FormatUint(uint64(end.Column+1), 10)
}
