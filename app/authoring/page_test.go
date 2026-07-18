package authoring

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAuthoringIsGoSXManagedBrowserWASM(t *testing.T) {
	contents := readEmbeddedSource(t, "page.gsx")
	for _, snippet := range []string{
		`<Surface`,
		`name="GotreesitterAuthoring"`,
		`runtime="go-wasm"`,
		`wasmPath={data.wasmURL}`,
		`requiredCapabilities="wasm"`,
		`id="ag-grammar"`,
		`id="ag-source"`,
		`id="ag-tree"`,
		`id="ag-errors"`,
		`id="ag-status"`,
		`aria-live="polite"`,
	} {
		if !strings.Contains(contents, snippet) {
			t.Errorf("page.gsx does not contain %q", snippet)
		}
	}
	// Note: unlike the playground's page_test.go, ".js" is deliberately not
	// in this forbidden list — "grammar.json" (this page's own subject
	// matter) contains that substring as a false positive.
	for _, forbidden := range []string{"<script", "ActionForm", "csrf", "method=", "action="} {
		if strings.Contains(contents, forbidden) {
			t.Errorf("page.gsx contains forbidden server/JS escape hatch %q", forbidden)
		}
	}
}

func TestAuthoringPrivacyCopyIsExplicit(t *testing.T) {
	contents := readEmbeddedSource(t, "page.gsx")
	if !strings.Contains(contents, "Nothing leaves this device") {
		t.Errorf("page.gsx does not state the browser-only privacy boundary")
	}
}

// TestAuthoringSeedGrammarIsValidJSON guards against an edit to
// initialGrammarJSON breaking its JSON syntax; the real import/generate
// round trip is exercised natively in internal/authoringengine.
func TestAuthoringSeedGrammarIsValidJSON(t *testing.T) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(initialGrammarJSON), &payload); err != nil {
		t.Fatalf("initialGrammarJSON is not valid JSON: %v", err)
	}
	if payload["name"] != "calc" {
		t.Errorf("initialGrammarJSON name: got %v, want %q", payload["name"], "calc")
	}
	if strings.TrimSpace(initialSample) == "" {
		t.Error("initialSample is empty")
	}
}

func readEmbeddedSource(t *testing.T, name string) string {
	t.Helper()
	data, err := pageSource.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
