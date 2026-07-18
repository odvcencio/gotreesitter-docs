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
		`id="ag-base"`,
		`id="ag-name"`,
		`id="ag-base-info"`,
		`id="ag-fidelity-note"`,
		`id="ag-editor-label"`,
		`baseIndexURL={data.baseIndexURL}`,
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

// TestAuthoringSeedDeltaIsValidJSON guards against an edit to
// initialDeltaJSON breaking its JSON syntax and its "delta on top of
// defaultBaseName" shape; the real base+delta merge -> import -> generate ->
// parse round trip (proving the delta-added "power" rule actually reaches
// the tree) is exercised natively in internal/authoringengine's merge tests
// and, end to end through the browser worker, by cmd/verify-authoring-browser.
func TestAuthoringSeedDeltaIsValidJSON(t *testing.T) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(initialDeltaJSON), &payload); err != nil {
		t.Fatalf("initialDeltaJSON is not valid JSON: %v", err)
	}
	rules, ok := payload["rules"].(map[string]any)
	if !ok {
		t.Fatalf("initialDeltaJSON has no \"rules\" object: %v", payload["rules"])
	}
	for _, want := range []string{"expression", "power"} {
		if _, ok := rules[want]; !ok {
			t.Errorf("initialDeltaJSON rules: missing %q (got keys %v)", want, rulesKeys(rules))
		}
	}
	if defaultBaseName != "calc" {
		t.Errorf("defaultBaseName: got %q, want %q", defaultBaseName, "calc")
	}
	if strings.TrimSpace(initialSample) == "" {
		t.Error("initialSample is empty")
	}
}

func rulesKeys(rules map[string]any) []string {
	keys := make([]string, 0, len(rules))
	for k := range rules {
		keys = append(keys, k)
	}
	return keys
}

func readEmbeddedSource(t *testing.T, name string) string {
	t.Helper()
	data, err := pageSource.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
