package docs

import (
	"strings"
	"testing"

	"m31labs.dev/gosx/islandtest"
)

func TestLangSearchIslandInitialRender(t *testing.T) {
	names := []string{"go", "json", "python", "rust", "typescript"}
	h, err := islandtest.New(LangSearchProgram(), langSearchProps(names))
	if err != nil {
		t.Fatalf("new harness: %v", err)
	}
	html := h.HTML()
	if !strings.Contains(html, "class=\"lang-island\"") {
		t.Fatalf("missing island root; got %s", html)
	}
	for _, name := range names {
		if !strings.Contains(html, ">"+name+"<") {
			t.Errorf("initial render missing tile %q; got %s", name, html)
		}
	}
	if strings.Contains(html, "langtile hidden") {
		t.Errorf("no tiles should be hidden with an empty query; got %s", html)
	}
	if !strings.Contains(compactHTML(html), `class="count mono"> 5 / 5 </span>`) {
		t.Errorf("expected count 5 / 5; got %s", html)
	}
}

func TestLangSearchIslandFiltersOnInput(t *testing.T) {
	names := []string{"go", "json", "python", "gomod", "rust"}
	h, err := islandtest.New(LangSearchProgram(), langSearchProps(names))
	if err != nil {
		t.Fatalf("new harness: %v", err)
	}
	if _, err := h.Input("setQuery", "go"); err != nil {
		t.Fatalf("dispatch setQuery: %v", err)
	}
	html := h.HTML()
	if !strings.Contains(compactHTML(html), `class="count mono"> 2 / 5 </span>`) {
		t.Errorf("expected count 2 / 5 after filtering to 'go'; got %s", html)
	}
	if !strings.Contains(html, ">go<") || !strings.Contains(html, ">gomod<") {
		t.Errorf("expected go and gomod tiles visible; got %s", html)
	}
	if strings.Contains(html, ">json<") {
		// "json" tile must be hidden — its lname text still exists in HTML
		// only inside a "langtile hidden" wrapper.
		if !strings.Contains(html, "langtile hidden") {
			t.Errorf("expected json tile to carry langtile hidden; got %s", html)
		}
	}
}

func compactHTML(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
