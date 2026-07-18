package authoringengine

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/odvcencio/gotreesitter/grammargen"
)

// MergeGrammarJSON implements Phase 2's "inheritance as data" — see the
// design doc's Part 3. base is a full tree-sitter grammar.json (one of the
// pre-generated assets under public/authoring/bases/, e.g. grammargen's own
// GoGrammar exported via grammargen.ExportGrammarJSON). delta is whatever the
// author typed into the grammar/delta editor: it may be a full grammar.json
// too, but normally it is a partial document — just the "rules" object
// containing the rules the author is adding or overriding, and optionally a
// subset of extras/conflicts/externals/inline/supertypes/word/precedences.
// An empty (or all-whitespace) delta is valid and simply reproduces base.
//
// This is deliberately NOT a raw JSON object merge (the "{...base,
// ...delta}" spread the design doc sketches as the client-side v1 shape). It
// is done at the *grammargen.Grammar level instead, reusing the package's own
// grammargen.ExtendGrammar — the exact clone-then-customize routine the
// engine already uses for Go-authored grammar inheritance (grammar.go:322).
// That buys three things a hand-rolled JSON merge would have to reimplement
// itself: a correct deep clone (so the cached base bytes are never mutated
// across repeated merges), the same rule-override-in-place/rule-append
// semantics ("the start rule stays first unless the delta redefines it" is
// exactly grammar.Define's existing behavior), and — the fidelity mitigation
// design risk #3 calls for — every one of base's exported LR-tuning fields
// (BinaryRepeatMode, the Prefer*Reduces flags, ExactPrefixStates,
// Precedences, ReservedWordSets, WantsForest, ...) carried into the merged
// grammar as data, not re-derived from a name lookup.
//
// nameOverride, if non-empty, replaces the merged grammar's "name" field
// (default: base's own name). See the package doc on why keeping base's name
// matters for fidelity: grammargen.applyImportGrammarShapeHints keys a
// handful of languages' import-time LR tuning off the literal grammar name;
// an inherited grammar that changes name only loses hints that exist for
// that literal string (of gotreesitter v0.40's headline authoring bases,
// only javascript/typescript carry any — and even those are carried forward
// here as already-resolved flags on the clone, not re-derived). A small
// number of deeper, generation-time checks in grammargen (elixir/c/cpp/objc/
// dart specific string comparisons) are NOT carried by this data-only merge;
// none of those languages are offered as authoring bases, but a future base
// addition could hit one, which is why the caller still surfaces a fidelity
// note whenever nameOverride differs from base's name — see
// cmd/authoring-wasm's updateFidelityNote.
func MergeGrammarJSON(baseJSON, deltaJSON []byte, nameOverride string) ([]byte, error) {
	base, err := grammargen.ImportGrammarJSON(baseJSON)
	if err != nil {
		return nil, fmt.Errorf("base grammar.json: %w", err)
	}
	delta, err := grammargen.ImportGrammarJSON(normalizeEmptyGrammarJSON(deltaJSON))
	if err != nil {
		return nil, fmt.Errorf("delta grammar.json: %w", err)
	}
	merged := mergeGrammar(base, delta, nameOverride)
	return grammargen.ExportGrammarJSON(merged)
}

// mergeGrammar is MergeGrammarJSON's Grammar-level core, split out so tests
// in this package can assert on the resulting *grammargen.Grammar's fields
// directly (ExportGrammarJSON does not round-trip every Go-only tuning
// field, so asserting through re-exported JSON bytes alone can't observe
// this function's most important behavior — see
// TestMergeGrammarJSONCarriesTuningFlags).
func mergeGrammar(base, delta *grammargen.Grammar, nameOverride string) *grammargen.Grammar {
	name := strings.TrimSpace(nameOverride)
	if name == "" {
		name = base.Name
	}

	return grammargen.ExtendGrammar(name, base, func(g *grammargen.Grammar) {
		// Rule keys delta shares with base override in place (Define does not
		// move an existing key's position in RuleOrder); new keys are
		// appended in delta's own declaration order.
		for _, ruleName := range delta.RuleOrder {
			g.Define(ruleName, delta.Rules[ruleName])
		}
		g.Extras = append(g.Extras, delta.Extras...)
		g.Externals = append(g.Externals, delta.Externals...)
		g.Conflicts = append(g.Conflicts, delta.Conflicts...)
		g.Inline = dedupeAppend(g.Inline, delta.Inline)
		g.Supertypes = dedupeAppend(g.Supertypes, delta.Supertypes)
		g.ReservedWordSets = append(g.ReservedWordSets, delta.ReservedWordSets...)
		if delta.Word != "" {
			g.Word = delta.Word
		}
		// Precedence *levels* are ordered top-to-bottom by rank; splicing two
		// independent level lists together would silently reorder ranks, so
		// a delta that defines any precedences replaces base's wholesale
		// rather than appending to it.
		if len(delta.Precedences) > 0 {
			g.Precedences = delta.Precedences
		}
		if delta.WantsForest {
			g.WantsForest = true
		}
	})
}

// GrammarSummary reports a grammar.json document's name and declared rule
// count — used by the base picker to show "go — 116 rules" once a base is
// fetched, before any delta has been applied.
func GrammarSummary(grammarJSON []byte) (name string, ruleCount int, err error) {
	g, err := grammargen.ImportGrammarJSON(grammarJSON)
	if err != nil {
		return "", 0, err
	}
	return g.Name, len(g.RuleOrder), nil
}

// normalizeEmptyGrammarJSON treats an empty/all-whitespace delta textarea as
// "{}" (a grammar.json document that defines nothing) rather than letting it
// fail json.Unmarshal outright — an author who picks a base and clears the
// delta editor should see the base unchanged, not an import error.
func normalizeEmptyGrammarJSON(data []byte) []byte {
	if len(bytes.TrimSpace(data)) == 0 {
		return []byte("{}")
	}
	return data
}

// dedupeAppend appends extra's entries onto existing, skipping any value
// already present (in either slice) so union-ing a base's and a delta's
// inline/supertypes lists doesn't duplicate entries the author didn't ask to
// repeat.
func dedupeAppend(existing, extra []string) []string {
	if len(extra) == 0 {
		return existing
	}
	seen := make(map[string]bool, len(existing)+len(extra))
	out := make([]string, 0, len(existing)+len(extra))
	for _, v := range existing {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, v := range extra {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
