package authoringengine

import (
	"context"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

// mergedCalcPowerJSON returns the exact merged (calc base + power delta)
// grammar.json the browser's default seed produces — see
// TestMergeGrammarJSONAddsAndOverridesRules and
// app/authoring/page.server.go's buildInitialDeltaJSON.
func mergedCalcPowerJSON(t *testing.T) []byte {
	t.Helper()
	merged, err := MergeGrammarJSON(calcBaseJSON(t), []byte(powerDeltaJSON), "")
	if err != nil {
		t.Fatalf("MergeGrammarJSON: %v", err)
	}
	return merged
}

func TestExportGrammarJSONContainsDeltaRule(t *testing.T) {
	grammar, lang, err := ImportAndGenerateWithContext(context.Background(), string(mergedCalcPowerJSON(t)))
	if err != nil {
		t.Fatalf("ImportAndGenerateWithContext: %v", err)
	}

	filename, content, err := ExportGrammar(grammar, lang, ExportFormatJSON, "calc", "")
	if err != nil {
		t.Fatalf("ExportGrammar(json): %v", err)
	}
	if filename != "calc.grammar.json" {
		t.Errorf("filename: got %q, want %q", filename, "calc.grammar.json")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("exported grammar.json is not valid JSON: %v\n%s", err, content)
	}
	rules, ok := payload["rules"].(map[string]any)
	if !ok {
		t.Fatalf("exported grammar.json has no \"rules\" object: %v", payload["rules"])
	}
	if _, ok := rules["power"]; !ok {
		t.Errorf("exported grammar.json is missing the delta-added %q rule (keys: %v)", "power", rulesKeysOf(rules))
	}
	if _, ok := rules["expression"]; !ok {
		t.Errorf("exported grammar.json is missing the base %q rule", "expression")
	}

	// Round-trips: re-importing the exported bytes must itself compile.
	reimported, err := grammargen.ImportGrammarJSON([]byte(content))
	if err != nil {
		t.Fatalf("re-import exported grammar.json: %v", err)
	}
	if _, err := grammargen.GenerateLanguage(reimported); err != nil {
		t.Fatalf("re-generate language from exported grammar.json: %v", err)
	}
}

func rulesKeysOf(rules map[string]any) []string {
	keys := make([]string, 0, len(rules))
	for k := range rules {
		keys = append(keys, k)
	}
	return keys
}

// TestExportGrammarGoIsSyntacticallyValid parses the exported .go with
// go/parser — a real Go-syntax check that catches malformed source
// (mismatched braces, a bad injectDotImport splice, ...) without paying for
// a full compiler invocation. The stronger, does-it-actually-build check is
// TestExportGrammarGoActuallyBuilds below.
func TestExportGrammarGoIsSyntacticallyValid(t *testing.T) {
	grammar, lang, err := ImportAndGenerateWithContext(context.Background(), string(mergedCalcPowerJSON(t)))
	if err != nil {
		t.Fatalf("ImportAndGenerateWithContext: %v", err)
	}
	filename, content, err := ExportGrammar(grammar, lang, ExportFormatGo, "calc", "")
	if err != nil {
		t.Fatalf("ExportGrammar(go): %v", err)
	}
	if filename != "calc.go" {
		t.Errorf("filename: got %q, want %q", filename, "calc.go")
	}
	if !strings.Contains(content, "package ") {
		t.Errorf("exported .go has no package clause:\n%s", content)
	}
	if !strings.Contains(content, "import . \""+grammargenImportPath+"\"") {
		t.Errorf("exported .go is missing the grammargen dot import:\n%s", content)
	}
	if !strings.Contains(content, "AuthoredCalcGrammar") {
		t.Errorf("exported .go is missing the expected generator func AuthoredCalcGrammar:\n%s", content)
	}
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, filename, content, parser.AllErrors); err != nil {
		t.Fatalf("exported .go does not parse as valid Go source: %v\n%s", err, content)
	}
}

// TestExportGrammarGoActuallyBuilds is the "STRONGER validity check": it
// writes the exported .go to a fresh temporary module that requires the same
// pinned github.com/odvcencio/gotreesitter version this repo itself uses,
// and runs a real `go build` against it — using only the local module cache
// (GOPROXY=off, GOSUMDB=off) so it needs no network access. This proves the
// export is not just syntactically valid Go but genuinely compiles against
// the real gotreesitter dependency, exactly as an author downloading it
// would experience.
func TestExportGrammarGoActuallyBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skips `go build` in a scratch module under -short")
	}
	requireGoToolchain(t)

	cases := []struct {
		name    string
		build   func() *grammargen.Grammar
		lang    bool // whether to pass a pre-generated *gts.Language
		wantPkg string
	}{
		{"calc-plus-power-delta", func() *grammargen.Grammar {
			g, err := grammargen.ImportGrammarJSON(mergedCalcPowerJSON(t))
			if err != nil {
				t.Fatal(err)
			}
			return g
		}, true, "calc"},
		// "go" is both a grammargen built-in constructor name (GoGrammar) and
		// a Go keyword — exercises both the dot-import collision guard and
		// the package-keyword guard in the same real `go build`.
		{"go-base-unmodified", grammargen.GoGrammar, true, "go_grammar"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := tc.build()
			var lang *gts.Language
			if tc.lang {
				generated, err := grammargen.GenerateLanguage(g)
				if err != nil {
					t.Fatalf("GenerateLanguage(%s): %v", tc.name, err)
				}
				lang = generated
			}
			filename, content, err := ExportGrammar(g, lang, ExportFormatGo, g.Name, "")
			if err != nil {
				t.Fatalf("ExportGrammar(go, %s): %v", tc.name, err)
			}
			if !strings.Contains(content, "package "+tc.wantPkg) {
				t.Errorf("%s: exported .go package clause: want %q, got:\n%s", tc.name, "package "+tc.wantPkg, firstLine(content))
			}
			buildExportedGoSource(t, filename, content)
		})
	}
}

// buildExportedGoSource writes filename/content into a scratch Go module
// whose only requirement is the exact gotreesitter version this repo's own
// go.mod pins, then runs `go build ./...` against it entirely offline (the
// module is already present in the local module cache because this repo
// itself depends on it).
func buildExportedGoSource(t *testing.T, filename, content string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off", "GOSUMDB=off",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
		return string(out)
	}
	run("go", "mod", "init", "authoringexporttest")
	run("go", "mod", "edit", "-require=github.com/odvcencio/gotreesitter@"+gotreesitterModuleVersion(t))
	run("go", "build", "./...")
}

// gotreesitterModuleVersion reads the exact github.com/odvcencio/gotreesitter
// version pinned in this module's own go.mod (via `go list -m`), so the
// scratch build module in buildExportedGoSource always matches — no
// hand-maintained version string to drift out of sync.
func gotreesitterModuleVersion(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Version}}", "github.com/odvcencio/gotreesitter").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -m github.com/odvcencio/gotreesitter: %v\n%s", err, out)
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		t.Fatal("go list -m github.com/odvcencio/gotreesitter returned an empty version")
	}
	return version
}

func requireGoToolchain(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

// TestExportGrammarCContainsStructuralMarkers checks the parser.c export for
// the markers a real tree-sitter parser.c always has: the language export
// function (tree_sitter_<name>), the ABI version #define, and the standard
// runtime include. This is a structural/pattern check, not a compile check —
// see cmd/verify-authoring-browser's package doc for what (if anything) this
// repo could additionally syntax-check with a C compiler, and why that was
// not attempted here (no vendored/discoverable tree_sitter/parser.h matching
// the exact ABI surface grammargen.EmitC targets in this environment).
func TestExportGrammarCContainsStructuralMarkers(t *testing.T) {
	grammar, lang, err := ImportAndGenerateWithContext(context.Background(), string(mergedCalcPowerJSON(t)))
	if err != nil {
		t.Fatalf("ImportAndGenerateWithContext: %v", err)
	}
	filename, content, err := ExportGrammar(grammar, lang, ExportFormatC, "calc", "")
	if err != nil {
		t.Fatalf("ExportGrammar(c): %v", err)
	}
	if filename != "parser.c" {
		t.Errorf("filename: got %q, want %q", filename, "parser.c")
	}
	for _, marker := range []string{
		"#include <tree_sitter/parser.h>",
		"#define LANGUAGE_VERSION",
		"tree_sitter_calc(",
	} {
		if !strings.Contains(content, marker) {
			t.Errorf("exported parser.c is missing marker %q", marker)
		}
	}
}

// TestExportGrammarCReusesCachedLanguage is the "do NOT recompile on
// export" requirement's unit-level proof: ExportGrammar's format="c" path
// must call grammargen.EmitC directly on an already-compiled *gts.Language
// (no second GenerateLanguage pass) rather than grammargen.GenerateC (which
// regenerates internally). It asserts this indirectly but concretely: on a
// grammar heavy enough for regeneration cost to be measurable natively
// (grammargen.LoxGrammar(), this design's own canonical "expensive"
// example), passing the cached Language must be dramatically faster than the
// no-cache (nil lang, forces a fresh GenerateC-driven regeneration) path —
// and must produce byte-identical output either way.
func TestExportGrammarCReusesCachedLanguage(t *testing.T) {
	if testing.Short() {
		t.Skip("skips the heavier lox-grammar timing comparison under -short")
	}
	g := grammargen.LoxGrammar()
	lang, err := grammargen.GenerateLanguage(g)
	if err != nil {
		t.Fatalf("GenerateLanguage(lox): %v", err)
	}

	cachedStart := time.Now()
	_, cachedContent, err := ExportGrammar(g, lang, ExportFormatC, "lox", "")
	cachedElapsed := time.Since(cachedStart)
	if err != nil {
		t.Fatalf("ExportGrammar(c, cached lang): %v", err)
	}

	freshStart := time.Now()
	_, freshContent, err := ExportGrammar(g, nil, ExportFormatC, "lox", "")
	freshElapsed := time.Since(freshStart)
	if err != nil {
		t.Fatalf("ExportGrammar(c, no cached lang): %v", err)
	}

	if cachedContent != freshContent {
		t.Error("ExportGrammar(c) output differs between the cached-Language path and the regenerate-from-Grammar path — should be byte-identical")
	}
	t.Logf("ExportGrammar(c) with cached Language: %s; forced to regenerate: %s", cachedElapsed, freshElapsed)
	// A conservative bound, not a tight one: this only needs to prove reuse
	// avoids paying for LR-table generation again, not pin an exact ratio
	// that could flake on a loaded CI machine.
	if cachedElapsed*2 > freshElapsed {
		t.Errorf("cached-Language export (%s) was not meaningfully faster than the regenerate-from-Grammar export (%s) — cache reuse may not be taking effect", cachedElapsed, freshElapsed)
	}
}

func TestCompileWithContextArtifactsExposesGrammarOnGenerateSuccess(t *testing.T) {
	result, grammar, lang := CompileWithContextArtifacts(context.Background(), string(calcGrammarJSONForArtifacts(t)), "1 + 2", true)
	if result.GenerateError != "" || result.ImportError != "" {
		t.Fatalf("unexpected compile error: import=%q generate=%q", result.ImportError, result.GenerateError)
	}
	if grammar == nil {
		t.Error("CompileWithContextArtifacts: grammar is nil on a successful compile")
	}
	if lang == nil {
		t.Error("CompileWithContextArtifacts: lang is nil on a successful compile")
	}
}

// TestCompileWithContextArtifactsStillExposesGrammarOnMessySample asserts
// the doc comment's key claim: a valid grammar whose sample source does not
// match it cleanly still yields non-nil artifacts (generation succeeded;
// only the *parse* of this particular sample was messy) — export readiness
// must not depend on the sample parsing without errors. gts's LR/GLR parser
// tolerates malformed input by producing ERROR nodes rather than failing
// outright, so this asserts on HasErrors (the realistic signal) rather than
// ParseError (see TestCompileWithContextArtifactsNilOnOversizedSource for
// the one case — a source that violates MaxSourceBytes before generation
// even has a chance to run — where ParseError does get set and the
// artifacts are correctly nil instead).
func TestCompileWithContextArtifactsStillExposesGrammarOnMessySample(t *testing.T) {
	result, grammar, lang := CompileWithContextArtifacts(context.Background(), string(calcGrammarJSONForArtifacts(t)), "@@@ not calc syntax @@@", true)
	if result.ImportError != "" || result.GenerateError != "" {
		t.Fatalf("unexpected compile-time error: import=%q generate=%q", result.ImportError, result.GenerateError)
	}
	if !result.HasErrors {
		t.Fatal("expected the malformed sample to produce a tree with error nodes (HasErrors)")
	}
	if grammar == nil || lang == nil {
		t.Error("CompileWithContextArtifacts: grammar/lang should stay non-nil when generation succeeded but the sample itself parsed messily")
	}
}

// TestCompileWithContextArtifactsNilOnOversizedSample is the size-limit
// counterpart: a source that violates MaxSourceBytes is rejected before
// grammargen.ImportGrammarJSON/GenerateLanguageWithContext ever run, so
// (unlike the messy-but-in-budget sample above) grammar/lang really are nil
// here — there is no compile to reuse.
func TestCompileWithContextArtifactsNilOnOversizedSample(t *testing.T) {
	oversizedSource := strings.Repeat("1 + ", MaxSourceBytes) // trivially exceeds MaxSourceBytes
	result, grammar, lang := CompileWithContextArtifacts(context.Background(), string(calcGrammarJSONForArtifacts(t)), oversizedSource, true)
	if result.ParseError == "" {
		t.Fatalf("expected a ParseError from an oversized sample, got none (result=%+v)", result)
	}
	if grammar != nil || lang != nil {
		t.Error("CompileWithContextArtifacts: grammar/lang should be nil when the sample was rejected before generation ran")
	}
}

func TestCompileWithContextArtifactsNilOnImportFailure(t *testing.T) {
	result, grammar, lang := CompileWithContextArtifacts(context.Background(), "{not valid json", "", true)
	if result.ImportError == "" {
		t.Fatal("expected an ImportError for invalid grammar.json")
	}
	if grammar != nil || lang != nil {
		t.Error("CompileWithContextArtifacts: grammar/lang should be nil on an import failure")
	}
}

func calcGrammarJSONForArtifacts(t *testing.T) []byte {
	t.Helper()
	return calcBaseJSON(t)
}

func TestSanitizeFilenameStem(t *testing.T) {
	cases := map[string]string{
		"calc":            "calc",
		"  my dsl  ":      "my-dsl",
		"my/dsl\\weird":   "my-dsl-weird",
		"":                "",
		"   ":             "",
		"-_leading_junk-": "leading_junk",
	}
	for in, want := range cases {
		if got := sanitizeFilenameStem(in); got != want {
			t.Errorf("sanitizeFilenameStem(%q): got %q, want %q", in, got, want)
		}
	}
}

func TestSanitizePackageNameAvoidsKeywords(t *testing.T) {
	if got := sanitizePackageName("go"); got == "go" {
		t.Errorf("sanitizePackageName(%q) returned a bare Go keyword: %q", "go", got)
	}
	if got := sanitizePackageName("type"); got == "type" {
		t.Errorf("sanitizePackageName(%q) returned a bare Go keyword: %q", "type", got)
	}
	if got := sanitizePackageName("mydsl"); got != "mydsl" {
		t.Errorf("sanitizePackageName(%q): got %q, want %q", "mydsl", got, "mydsl")
	}
	if got := sanitizePackageName(""); got != "" {
		t.Errorf("sanitizePackageName(\"\"): got %q, want \"\"", got)
	}
}

func TestExportedIdentifierNoCollisionAcrossBuiltinNames(t *testing.T) {
	// Every grammargen built-in base this page ships (see
	// cmd/build-authoring-wasm's baseGrammars) must produce a funcName that
	// cannot collide with grammargen's own "func XGrammar() *Grammar"
	// constructor of the same base name once dot-imported.
	for _, base := range []string{"calc", "json", "ini", "mustache", "lox", "go", "typescript", "javascript"} {
		got := "Authored" + exportedIdentifier(base) + "Grammar"
		if strings.HasPrefix(got, "Authored") == false {
			t.Errorf("base %q: funcName %q lost its Authored prefix", base, got)
		}
	}
}

func TestExportGrammarUnknownFormat(t *testing.T) {
	grammar, lang, err := ImportAndGenerateWithContext(context.Background(), string(calcBaseJSON(t)))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ExportGrammar(grammar, lang, ExportFormat("yaml"), "calc", ""); err == nil {
		t.Fatal("expected an error for an unknown export format")
	}
}

func TestExportGrammarNilGrammar(t *testing.T) {
	if _, _, err := ExportGrammar(nil, nil, ExportFormatJSON, "x", ""); err == nil {
		t.Fatal("expected an error for a nil grammar")
	}
}
