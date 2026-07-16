package playground

import (
	"strings"
	"testing"
)

func TestPlaygroundIsGoSXManagedBrowserWASM(t *testing.T) {
	contents := readEmbeddedSource(t, "page.gsx")
	for _, snippet := range []string{
		`<Surface`,
		`name="GotreesitterPlayground"`,
		`runtime="go-wasm"`,
		`wasmPath={data.wasmURL}`,
		`grammarIndexURL={data.grammarIndexURL}`,
		`requiredCapabilities="wasm fetch"`,
		`type="button"`,
		`aria-live="polite"`,
	} {
		if !strings.Contains(contents, snippet) {
			t.Errorf("page.gsx does not contain %q", snippet)
		}
	}
	for _, forbidden := range []string{"<script", ".js", "ActionForm", "csrf", "method=", "action="} {
		if strings.Contains(contents, forbidden) {
			t.Errorf("page.gsx contains forbidden server/JS escape hatch %q", forbidden)
		}
	}
}

func TestPlaygroundPrivacyCopyIsExplicit(t *testing.T) {
	contents := readEmbeddedSource(t, "page.gsx")
	for _, statement := range []string{
		"stays entirely on this device",
		"never submitted to the server",
		"never editor contents",
	} {
		if !strings.Contains(contents, statement) {
			t.Errorf("page.gsx does not state %q", statement)
		}
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
