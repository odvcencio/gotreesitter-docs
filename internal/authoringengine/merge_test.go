package authoringengine

import (
	"strings"
	"testing"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

// powerDeltaJSON overrides calc's "expression" rule to add one more
// alternative (a reference to a brand-new "power" rule) and defines that new
// rule — the same shape as app/authoring/page.server.go's seed delta. It
// exercises the two things a real author does when inheriting a base: wire a
// new rule into an existing choice point, and define that new rule.
const powerDeltaJSON = `{
  "rules": {
    "expression": {
      "type": "CHOICE",
      "members": [
        {"name": "number", "type": "SYMBOL"},
        {"name": "power", "type": "SYMBOL"}
      ]
    },
    "power": {
      "type": "PREC_RIGHT",
      "value": 4,
      "content": {
        "type": "SEQ",
        "members": [
          {"type": "FIELD", "name": "left", "content": {"type": "SYMBOL", "name": "expression"}},
          {"type": "FIELD", "name": "operator", "content": {"type": "STRING", "value": "**"}},
          {"type": "FIELD", "name": "right", "content": {"type": "SYMBOL", "name": "expression"}}
        ]
      }
    }
  }
}`

func calcBaseJSON(t *testing.T) []byte {
	t.Helper()
	data, err := grammargen.ExportGrammarJSON(grammargen.CalcGrammar())
	if err != nil {
		t.Fatalf("export calc base: %v", err)
	}
	return data
}

func TestMergeGrammarJSONAddsAndOverridesRules(t *testing.T) {
	base := calcBaseJSON(t)

	merged, err := MergeGrammarJSON(base, []byte(powerDeltaJSON), "")
	if err != nil {
		t.Fatalf("MergeGrammarJSON: %v", err)
	}

	g, err := grammargen.ImportGrammarJSON(merged)
	if err != nil {
		t.Fatalf("re-import merged grammar.json: %v", err)
	}

	if g.Name != "calc" {
		t.Errorf("merged name: got %q, want %q (base's name kept by default)", g.Name, "calc")
	}

	// "program" (untouched by the delta) must still be present and stay
	// first — base's start rule survives the merge.
	if len(g.RuleOrder) == 0 || g.RuleOrder[0] != "program" {
		t.Errorf("merged RuleOrder[0]: got %v, want %q first", g.RuleOrder, "program")
	}
	// "expression" (overridden by the delta) keeps its base position
	// (Define never moves an existing key).
	wantExpressionIdx := 1
	if idx := indexOf(g.RuleOrder, "expression"); idx != wantExpressionIdx {
		t.Errorf("merged RuleOrder index of %q: got %d, want %d (%v)", "expression", idx, wantExpressionIdx, g.RuleOrder)
	}
	// "power" (new in the delta) is appended.
	if idx := indexOf(g.RuleOrder, "power"); idx != len(g.RuleOrder)-1 {
		t.Errorf("merged RuleOrder index of %q: got %d, want last (%v)", "power", idx, g.RuleOrder)
	}
	// "number" (untouched by the delta, base-only) must still be reachable —
	// the delta's "expression" override kept the SYMBOL reference to it.
	if _, ok := g.Rules["number"]; !ok {
		t.Error("merged grammar lost base's \"number\" rule")
	}

	lang, err := grammargen.GenerateLanguage(g)
	if err != nil {
		t.Fatalf("generate merged language: %v", err)
	}
	parser := gts.NewParser(lang)
	tree, err := parser.Parse([]byte("2 ** 3 + 1"))
	if err != nil {
		t.Fatalf("parse sample against merged language: %v", err)
	}
	defer tree.Release()

	sexp := tree.RootNode().SExpr(lang)
	if !strings.Contains(sexp, "(power") {
		t.Errorf("merged tree does not contain the delta-added %q rule: %s", "power", sexp)
	}
	if !strings.Contains(sexp, "(expression") {
		t.Errorf("merged tree does not contain a base %q rule: %s", "expression", sexp)
	}
	if !strings.Contains(sexp, "(number") {
		t.Errorf("merged tree does not contain a base %q rule: %s", "number", sexp)
	}
}

func TestMergeGrammarJSONNameOverride(t *testing.T) {
	base := calcBaseJSON(t)
	merged, err := MergeGrammarJSON(base, []byte(powerDeltaJSON), "my-calc")
	if err != nil {
		t.Fatalf("MergeGrammarJSON: %v", err)
	}
	g, err := grammargen.ImportGrammarJSON(merged)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if g.Name != "my-calc" {
		t.Errorf("merged name: got %q, want %q", g.Name, "my-calc")
	}
}

func TestMergeGrammarJSONEmptyDeltaReproducesBase(t *testing.T) {
	base := calcBaseJSON(t)
	baseGrammar, err := grammargen.ImportGrammarJSON(base)
	if err != nil {
		t.Fatalf("import base: %v", err)
	}

	for _, delta := range []string{"", "   \n\t  ", "{}"} {
		merged, err := MergeGrammarJSON(base, []byte(delta), "")
		if err != nil {
			t.Fatalf("MergeGrammarJSON(delta=%q): %v", delta, err)
		}
		g, err := grammargen.ImportGrammarJSON(merged)
		if err != nil {
			t.Fatalf("re-import merged (delta=%q): %v", delta, err)
		}
		if len(g.RuleOrder) != len(baseGrammar.RuleOrder) {
			t.Errorf("delta=%q: merged has %d rules, want %d (unchanged base)", delta, len(g.RuleOrder), len(baseGrammar.RuleOrder))
		}
		if g.Name != baseGrammar.Name {
			t.Errorf("delta=%q: merged name %q, want base's %q", delta, g.Name, baseGrammar.Name)
		}
	}
}

func TestMergeGrammarJSONUnionsExtrasConflictsExternals(t *testing.T) {
	base := calcBaseJSON(t)
	delta := `{
	  "rules": {},
	  "extras": [{"type": "STRING", "value": "#comment"}],
	  "conflicts": [["expression"]]
	}`
	merged, err := MergeGrammarJSON(base, []byte(delta), "")
	if err != nil {
		t.Fatalf("MergeGrammarJSON: %v", err)
	}
	g, err := grammargen.ImportGrammarJSON(merged)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if len(g.Extras) != 2 { // base's "\s" pattern + delta's "#comment" string
		t.Errorf("merged Extras: got %d entries, want 2", len(g.Extras))
	}
	if len(g.Conflicts) != 1 || len(g.Conflicts[0]) != 1 || g.Conflicts[0][0] != "expression" {
		t.Errorf("merged Conflicts: got %v, want [[\"expression\"]]", g.Conflicts)
	}
}

// TestMergeGrammarJSONCarriesTuningFlags is the risk-#3 fidelity assertion:
// a merge that renames the result must still carry base's already-resolved
// Go-only LR-tuning fields (BinaryRepeatMode, ExactPrefixStates, ...) into
// the merged Grammar. It asserts on the *grammargen.Grammar directly (via
// the package-internal mergeGrammar) rather than round-tripping through
// ExportGrammarJSON/ImportGrammarJSON, because ExportGrammarJSON does not
// serialize these fields at all — a JSON-only assertion could not observe
// this behavior (or its absence).
func TestMergeGrammarJSONCarriesTuningFlags(t *testing.T) {
	base := grammargen.NewGrammar("javascript")
	base.Define("start", grammargen.Str("x"))
	// Simulates what applyImportGrammarShapeHints sets for a real
	// "javascript" grammar.json import (grammargen/import_grammarjson.go's
	// `case "javascript", "typescript", ...:` branch) — set directly here so
	// this test doesn't depend on that switch's current contents.
	base.BinaryRepeatMode = true
	base.ExactPrefixStates = 999999

	delta := grammargen.NewGrammar("")
	delta.Define("extra", grammargen.Str("y"))

	merged := mergeGrammar(base, delta, "my-javascript")

	if merged.Name != "my-javascript" {
		t.Errorf("merged.Name: got %q, want %q", merged.Name, "my-javascript")
	}
	if !merged.BinaryRepeatMode {
		t.Error("merged.BinaryRepeatMode: got false, want true (base's tuning flag was not carried through the rename)")
	}
	if merged.ExactPrefixStates != 999999 {
		t.Errorf("merged.ExactPrefixStates: got %d, want %d", merged.ExactPrefixStates, 999999)
	}
	if _, ok := merged.Rules["extra"]; !ok {
		t.Error("merged grammar lost the delta's \"extra\" rule")
	}
	if _, ok := merged.Rules["start"]; !ok {
		t.Error("merged grammar lost the base's \"start\" rule")
	}
}

func TestMergeGrammarJSONRejectsInvalidDelta(t *testing.T) {
	base := calcBaseJSON(t)
	if _, err := MergeGrammarJSON(base, []byte("{not valid json"), ""); err == nil {
		t.Fatal("expected an error for invalid delta JSON")
	}
}

func TestMergeGrammarJSONRejectsInvalidBase(t *testing.T) {
	if _, err := MergeGrammarJSON([]byte("{not valid json"), []byte("{}"), ""); err == nil {
		t.Fatal("expected an error for invalid base JSON")
	}
}

func indexOf(s []string, v string) int {
	for i, item := range s {
		if item == v {
			return i
		}
	}
	return -1
}
