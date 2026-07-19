package playgroundengine

import (
	"sort"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// MaxHighlightSpans caps the highlight overlay the same way MaxTreeRows caps
// the tree view: a pathological source (or one with an enormous number of
// tiny tokens) must not balloon this call's DOM payload.
const MaxHighlightSpans = 4000

// HighlightSpan is one styled byte range of source, drawn from the loaded
// grammar's own highlight query (grammars.LangEntry.HighlightQuery — a real
// highlights.scm-derived query gotreesitter validates, at the pinned
// release, for every one of its 206 grammars: see
// TestAllHighlightQueriesCompile/TestHighlightQueriesProduceResults in
// github.com/odvcencio/gotreesitter's grammars package). Class is one of the
// site's tk-* palette classes (public/docs.css, the same classes
// app/highlight.go's server-side markdown-fence classifier uses). Spans
// whose capture family has no confident tk-* mapping are omitted by
// ComputeHighlight entirely, rather than guessed at — the caller renders
// that byte range unstyled instead ("real highlighting or none").
type HighlightSpan struct {
	StartByte int
	EndByte   int
	Class     string
}

// ComputeHighlight runs lang's own highlight query (resolved by
// languageName through the grammars registry — the same registry
// ParseLanguage already consults for TokenSourceFactory) against source and
// returns tk-*-styled spans in byte order. notice explains why spans came
// back empty when that isn't simply "this source has nothing a highlighted
// rule matches"; callers should treat a non-empty notice as informational,
// not an error — the plain, unstyled source is still a complete, honest
// result.
//
// Every one of gotreesitter's 206 shipped grammars carries a real
// highlights.scm-derived query baked into the registry's LangEntry.HighlightQuery
// Go string constants, which this build already links in (cmd/playground-wasm
// imports the grammars package for ExternalScanner/ExternalLexStates
// lookups) — so reaching all 206 languages needs no extra network fetch and
// no per-language special-casing here.
//
// This runs the query and resolves overlaps itself (mirroring
// gotreesitter.Highlighter's own stack-sweep algorithm — see
// resolveHighlightCandidates) rather than delegating to
// gts.NewHighlighter/Highlighter.Highlight, so it can adapt the real query's
// overlap resolution to this build's smaller tk-* palette without losing
// genuine nesting behavior — see resolveHighlightCandidates's doc comment
// for the one deliberate tie-break deviation this requires (go's shipped
// query is the concrete case that motivated it).
func ComputeHighlight(source, languageName string, lang *gts.Language) (spans []HighlightSpan, notice string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			spans = nil
			notice = "highlight generation panicked"
		}
	}()
	if lang == nil {
		return nil, "language is not loaded"
	}
	if len(source) > MaxSourceBytes {
		return nil, "source is too large to highlight"
	}

	entry := grammars.DetectLanguageByName(strings.TrimSpace(languageName))
	if entry == nil || strings.TrimSpace(entry.HighlightQuery) == "" {
		return nil, "no highlight query is bundled for this grammar"
	}

	query, err := gts.NewQuery(entry.HighlightQuery, lang)
	if err != nil {
		return nil, "highlight query failed to compile: " + err.Error()
	}

	src := []byte(source)
	parser := gts.NewParser(lang)
	var tree *gts.Tree
	if entry.TokenSourceFactory != nil {
		factory := entry.TokenSourceFactory
		tree, err = parser.ParseWithTokenSourceFactory(src, func(s []byte) (gts.TokenSource, error) {
			return factory(s, lang), nil
		})
	} else {
		tree, err = parser.Parse(src)
	}
	if err != nil || tree == nil || tree.RootNode() == nil {
		return nil, "source failed to parse for highlighting"
	}
	defer tree.Release()

	// Every real capture goes into the resolution pool, including ones with
	// no tk-* mapping (class == "") — an unmapped-but-real capture (e.g. the
	// interpolated expression inside a JS/TS template string, which the
	// query deliberately leaves uncaptured-as-string so it isn't painted
	// tk-str along with the surrounding literal text) still has to be able
	// to carve its hole out of a wider active range during resolution.
	// Dropping it here instead of at emit time would let that wider range
	// leak across the hole and mis-paint real code as if it were string
	// content. See resolveHighlightCandidates for the one case where an
	// unmapped capture is deliberately NOT allowed to win: an exact tie
	// against a mapped capture at the identical byte range.
	var candidates []highlightCandidate
	for _, match := range query.Execute(tree) {
		for _, capture := range match.Captures {
			if capture.Node == nil {
				continue
			}
			start, end := capture.Node.StartByte(), capture.Node.EndByte()
			if end <= start {
				continue
			}
			candidates = append(candidates, highlightCandidate{
				start: start, end: end,
				class:        tkClassForCapture(capture.Name),
				patternIndex: match.PatternIndex,
			})
		}
	}
	if len(candidates) == 0 {
		return nil, ""
	}

	spans = resolveHighlightCandidates(candidates)
	if len(spans) > MaxHighlightSpans {
		spans = spans[:MaxHighlightSpans]
	}
	if len(spans) == 0 {
		return nil, ""
	}
	return spans, ""
}

// highlightCandidate is one real capture (class is "" when tkClassForCapture
// found no confident tk-* mapping), still subject to overlap resolution
// against its siblings.
type highlightCandidate struct {
	start, end   uint32
	class        string
	patternIndex int
}

// resolveHighlightCandidates sorts candidates by (start asc, width desc,
// [exact-range ties only] mapped-before-unmapped, patternIndex asc) and
// sweeps them with a stack, mirroring gotreesitter.Highlighter's own
// resolveOverlapsInto: at any position the top of the stack (narrowest
// active range) is what gets emitted, and genuine nesting always wins
// regardless of mapped-ness — that's what lets an unmapped-but-real capture
// (e.g. a template string's interpolated expression) still carve its hole
// out of a wider mapped range around it (see ComputeHighlight's doc
// comment).
//
// The one deliberate deviation from the real Highlighter is the tie-break
// for an EXACT range tie (identical start AND end — the same node matched
// by two different patterns in the same query, not nesting): the real
// Highlighter always lets the textually-last-declared pattern win. Several
// shipped queries (go's among them) place a broad, unmapped catch-all
// pattern like "(identifier) @variable" after specific, mapped ones like
// "(function_declaration name: (identifier) @function)", so obeying
// declaration order there would silently discard color this build could
// have shown. Since an unmapped capture is invisible either way, preferring
// whichever exact-tied candidate IS mapped can only produce a more (never
// less) informative result; only among same-mappedness ties does
// declaration order still apply.
func resolveHighlightCandidates(candidates []highlightCandidate) []HighlightSpan {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].start != candidates[j].start {
			return candidates[i].start < candidates[j].start
		}
		wi := candidates[i].end - candidates[i].start
		wj := candidates[j].end - candidates[j].start
		if wi != wj {
			return wi > wj // widest first, so narrower ranges sort later (end up on top of the stack)
		}
		mi, mj := candidates[i].class != "", candidates[j].class != ""
		if mi != mj {
			return !mi // exact tie: the unmapped one sorts first, so the mapped one ends up on top
		}
		return candidates[i].patternIndex < candidates[j].patternIndex // last-declared pattern ends up on top of the stack
	})

	const noBoundary = ^uint32(0)
	emit := func(spans []HighlightSpan, start, end uint32, class string) []HighlightSpan {
		if end <= start || class == "" {
			return spans
		}
		if n := len(spans); n > 0 && spans[n-1].Class == class && uint32(spans[n-1].EndByte) == start {
			spans[n-1].EndByte = int(end)
			return spans
		}
		return append(spans, HighlightSpan{StartByte: int(start), EndByte: int(end), Class: class})
	}

	var spans []HighlightSpan
	var stack []highlightCandidate
	idx := 0
	pos := uint32(0)
	if len(candidates) > 0 {
		pos = candidates[0].start
	}
	for idx < len(candidates) || len(stack) > 0 {
		nextStart := noBoundary
		if idx < len(candidates) {
			nextStart = candidates[idx].start
		}
		nextEnd := noBoundary
		if len(stack) > 0 {
			nextEnd = stack[len(stack)-1].end
		}
		next := nextStart
		if nextEnd < next {
			next = nextEnd
		}

		if len(stack) > 0 && next > pos {
			spans = emit(spans, pos, next, stack[len(stack)-1].class)
			pos = next
		} else if pos < next {
			pos = next
		}

		for len(stack) > 0 && stack[len(stack)-1].end <= pos {
			stack = stack[:len(stack)-1]
		}
		for idx < len(candidates) && candidates[idx].start == pos {
			stack = append(stack, candidates[idx])
			idx++
		}
		if len(stack) == 0 && idx < len(candidates) && pos < candidates[idx].start {
			pos = candidates[idx].start
		}
		if len(stack) > 0 {
			if pos < stack[len(stack)-1].start {
				pos = stack[len(stack)-1].start
			}
			for len(stack) > 0 && pos >= stack[len(stack)-1].end {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return spans
}

// tkClassForCapture maps a tree-sitter highlight capture name (e.g.
// "function.builtin", "keyword.operator") to one of the site's tk-* palette
// classes. Only the leading dot-segment ("family") is consulted, except for
// "keyword.operator" which needs its own case since bare "keyword" and
// "operator" map to different colors. Predicate-only helper captures (a
// leading "_", e.g. "@_type" referenced only inside a query's own
// #eq?/#match? predicate, never meant to be displayed) and any family this
// table doesn't recognize both return "" — the caller leaves that span's
// source unstyled rather than guessing.
func tkClassForCapture(capture string) string {
	if capture == "keyword.operator" {
		return "tk-op"
	}
	family := capture
	if idx := strings.IndexByte(capture, '.'); idx >= 0 {
		family = capture[:idx]
	}
	if family == "" || strings.HasPrefix(family, "_") {
		return ""
	}
	switch family {
	case "comment":
		return "tk-cm"
	case "string", "character", "regexp", "embedded":
		return "tk-str"
	case "number", "float":
		return "tk-num"
	case "boolean", "constant":
		return "tk-kw"
	case "keyword", "conditional", "repeat", "exception", "include", "preproc",
		"storageclass", "storage", "label", "define", "use", "import", "var",
		"return", "throw", "catch", "finally", "try", "for", "while",
		"synchronized", "local":
		return "tk-kw"
	case "type", "structure", "union", "interface", "protocol",
		"annotation", "attribute", "constructor", "tag":
		return "tk-ty"
	case "function", "method":
		return "tk-fn"
	case "punctuation", "delimiter":
		return "tk-pn"
	case "operator":
		return "tk-op"
	}
	return ""
}
