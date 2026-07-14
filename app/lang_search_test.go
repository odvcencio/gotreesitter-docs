package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/gosx"
)

func TestLangSearchIsAStandardGoEngineSurface(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("docs", "__slug", "page.gsx"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		`name="GoTreesitterLangSearch"`,
		`runtime="go-wasm"`,
		`wasmPath={data.langSearchWasmURL}`,
		`requiredCapabilities="wasm"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("language search source is missing %q", want)
		}
	}
	if strings.Contains(text, "//gosx:island") || strings.Contains(text, "m31labs.dev/gosx/signal") {
		t.Fatal("language search still depends on the island runtime")
	}

	compiled, err := gosx.Compile(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, component := range compiled.Components {
		if component.Name == "LangSearch" && component.IsIsland {
			t.Fatal("LangSearch compiled as an island")
		}
	}
}

func TestLangSearchTokenSourcesPreserveCatalogOrder(t *testing.T) {
	names := []string{"go", "markdown", "python", "typescript"}
	got := LangSearchTokenSources(names)
	for index := 1; index < len(got); index++ {
		previous := indexOf(names, got[index-1])
		current := indexOf(names, got[index])
		if previous < 0 || current < 0 || previous >= current {
			t.Fatalf("token-source order %v does not preserve catalog order %v", got, names)
		}
	}
}

func indexOf(items []string, want string) int {
	for index, item := range items {
		if item == want {
			return index
		}
	}
	return -1
}
