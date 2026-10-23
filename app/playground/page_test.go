package playground

import (
	"net/url"
	"strings"
	"testing"

	"github.com/odvcencio/gotreesitter-docs/internal/playgroundshare"
)

func TestInitialPlaygroundSamplePreselectsOnlyIndexedLanguages(t *testing.T) {
	language, _, sample := initialPlaygroundSample("rust", "")
	if language != "rust" {
		t.Fatalf("initial language = %q, want rust", language)
	}
	if !strings.Contains(sample.Source, "struct Greeter") {
		t.Fatalf("Rust starter sample = %q, want the Rust sample", sample.Source)
	}
	if !strings.Contains(sample.Query, "function_item") {
		t.Fatalf("Rust query = %q, want a Rust function_item query", sample.Query)
	}

	for _, invalid := range []string{"Rust", "not-a-language", "package main"} {
		language, _, sample = initialPlaygroundSample(invalid, "")
		if language != "go" {
			t.Errorf("initial language for %q = %q, want go", invalid, language)
		}
		if !strings.Contains(sample.Source, "package main") {
			t.Errorf("initial Go sample for %q = %q", invalid, sample.Source)
		}
	}
}

func TestInitialPlaygroundSampleHonoursNamedSamples(t *testing.T) {
	language, name, sample := initialPlaygroundSample("go", "add")
	if language != "go" || name != "add" || !strings.Contains(sample.Source, "func add(left, right int)") {
		t.Errorf("go/add = %q, %q, %q", language, name, sample.Source)
	}
	if _, name, sample = initialPlaygroundSample("html", "add"); name != "add" || !strings.Contains(sample.Source, "<article>") {
		t.Errorf("html/add = %q, %q", name, sample.Source)
	}
	// Lua has no add sample: it falls back to the comment probe, reported as greet.
	if _, name, sample = initialPlaygroundSample("lua", "add"); name != "greet" || strings.Contains(sample.Source, "add") {
		t.Errorf("lua/add = %q, %q", name, sample.Source)
	}
	if _, name, sample = initialPlaygroundSample("go", "nope"); name != "greet" || !strings.Contains(sample.Source, "func greet") {
		t.Errorf("go/nope = %q, %q", name, sample.Source)
	}
}

func TestPlaygroundIsGoSXManagedBrowserWASM(t *testing.T) {
	contents := readEmbeddedSource(t, "page.gsx")
	for _, snippet := range []string{
		`<Surface`,
		`name="GotreesitterPlayground"`,
		`runtime="go-wasm"`,
		`wasmPath={data.wasmURL}`,
		`grammarIndexURL={data.grammarIndexURL}`,
		`requiredCapabilities="wasm fetch"`,
		`capabilities="wasm fetch clipboard"`,
		`pg-share`,
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

// The text-input capability makes GoSX focus a hidden contenteditable on mount,
// which scrolled the page down on load. The playground has real textareas.
func TestPlaygroundDoesNotRequestTheTextInputCapability(t *testing.T) {
	if strings.Contains(readEmbeddedSource(t, "page.gsx"), "text-input") {
		t.Error("page.gsx requests text-input; GoSX would focus a hidden element on load and scroll the page")
	}
}

func TestPlaygroundPrivacyCopyIsExplicit(t *testing.T) {
	contents := readEmbeddedSource(t, "page.gsx")
	for _, statement := range []string{
		"stays entirely on this device",
		"never submitted to the server unless you copy a share link",
		"never editor contents",
		"encodes your source in the URL",
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

func TestResolvePlaygroundStatePrefersAValidShare(t *testing.T) {
	values, err := playgroundshare.Encode(playgroundshare.Share{Language: "go", Source: "package main\nfunc shared() {}\n", Query: "(function_declaration) @f"})
	if err != nil {
		t.Fatal(err)
	}
	state := resolvePlaygroundState("go", "add", values)
	if !state.Shared || state.ShareError != "" || state.Language != "go" {
		t.Fatalf("state = %+v", state)
	}
	if !strings.Contains(state.Source, "func shared()") || state.Query != "(function_declaration) @f" {
		t.Fatalf("shared source/query not used: %+v", state)
	}
}

func TestResolvePlaygroundStateFallsBackOnABrokenShare(t *testing.T) {
	values := url.Values{"lang": {"go"}, "s": {"1.%%%"}}
	state := resolvePlaygroundState("go", "add", values)
	if state.Shared || state.ShareError == "" {
		t.Fatalf("a broken share must report an error: %+v", state)
	}
	if !strings.Contains(state.Source, "func add(left, right int)") {
		t.Fatalf("a broken share must fall back to the sample: %q", state.Source)
	}
}

func TestResolvePlaygroundStateWithoutAShare(t *testing.T) {
	state := resolvePlaygroundState("python", "", url.Values{"lang": {"python"}})
	if state.Shared || state.ShareError != "" || !strings.Contains(state.Source, "def greet") {
		t.Fatalf("state = %+v", state)
	}
}

func TestResolvePlaygroundStateIgnoresUnknownShareLanguages(t *testing.T) {
	values, err := playgroundshare.Encode(playgroundshare.Share{Language: "not-a-language", Source: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if state := resolvePlaygroundState("not-a-language", "", values); state.Language != "go" {
		t.Fatalf("language = %q, want go", state.Language)
	}
}

func TestSharedEmptyQueryRendersAnEmptyQueryEditor(t *testing.T) {
	values, err := playgroundshare.Encode(playgroundshare.Share{Language: "go", Source: "package main\n", Query: ""})
	if err != nil {
		t.Fatal(err)
	}
	state := resolvePlaygroundState("go", "add", values)
	if !state.Shared || state.Query != "" {
		t.Fatalf("shared empty query = %q (shared=%v), want empty", state.Query, state.Shared)
	}
	// Without a share payload the sample's starter query still applies.
	if plain := resolvePlaygroundState("go", "add", url.Values{}); plain.Query == "" {
		t.Fatal("a link without a share keeps the starter query")
	}
}
