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
	if !strings.Contains(html, `class="count mono">5 / 206<`) {
		t.Errorf("expected count 5 / 206; got %s", html)
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
	if !strings.Contains(html, `class="count mono">2 / 206<`) {
		t.Errorf("expected count 2 / 206 after filtering to 'go'; got %s", html)
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

func TestPlaygroundIslandInitialRender(t *testing.T) {
	h, err := islandtest.New(PlaygroundProgram(), nil)
	if err != nil {
		t.Fatalf("new harness: %v", err)
	}
	html := h.HTML()
	if !strings.Contains(html, "class=\"playground-island\"") {
		t.Fatalf("missing island root; got %s", html)
	}
	if !strings.Contains(html, "segbtn on") {
		t.Errorf("expected the default sample's seg button to carry 'on'; got %s", html)
	}
	if !strings.Contains(html, "source_file") {
		t.Errorf("expected the go sample's tree (source_file root) by default; got %s", html)
	}
	if strings.Contains(html, "document") || strings.Contains(html, "module") {
		t.Errorf("json/python trees must not render while sample=go; got %s", html)
	}
	if !strings.Contains(html, "Type an edit") {
		t.Errorf("expected the not-edited toggle label by default; got %s", html)
	}
}

func TestPlaygroundIslandSampleSwitch(t *testing.T) {
	h, err := islandtest.New(PlaygroundProgram(), nil)
	if err != nil {
		t.Fatalf("new harness: %v", err)
	}
	if _, err := h.Click("select_json"); err != nil {
		t.Fatalf("dispatch select_json: %v", err)
	}
	html := h.HTML()
	if !strings.Contains(html, "\"document\"") && !strings.Contains(html, ">document<") {
		t.Errorf("expected json sample's tree (document root) after switching; got %s", html)
	}
	if strings.Contains(html, "source_file") {
		t.Errorf("go sample's tree must not render after switching to json; got %s", html)
	}
}

func TestPlaygroundIslandQueryHighlightsAndBadges(t *testing.T) {
	h, err := islandtest.New(PlaygroundProgram(), nil)
	if err != nil {
		t.Fatalf("new harness: %v", err)
	}
	if _, err := h.Input("setQuery", "(identifier) @name"); err != nil {
		t.Fatalf("dispatch setQuery: %v", err)
	}
	html := h.HTML()
	if !strings.Contains(html, "trow qmatch") {
		t.Errorf("expected at least one qmatch row for query '(identifier) @name'; got %s", html)
	}
	if !strings.Contains(html, "qcap") || !strings.Contains(html, "@name") {
		t.Errorf("expected a @name capture badge; got %s", html)
	}
	if !strings.Contains(html, "matched") {
		t.Errorf("expected the match count label; got %s", html)
	}
	if strings.Contains(html, ">0</span> matched") {
		t.Errorf("expected a non-zero match count for '(identifier)' against the go sample; got %s", html)
	}
}

func TestPlaygroundIslandIncrementalToggle(t *testing.T) {
	h, err := islandtest.New(PlaygroundProgram(), nil)
	if err != nil {
		t.Fatalf("new harness: %v", err)
	}
	before := h.HTML()
	if strings.Contains(before, "trow reused") || strings.Contains(before, "trow edited") {
		t.Fatalf("marks must not show before 'edited' is toggled on; got %s", before)
	}
	if _, err := h.Click("toggleEdit"); err != nil {
		t.Fatalf("dispatch toggleEdit: %v", err)
	}
	after := h.HTML()
	if !strings.Contains(after, "trow reused") {
		t.Errorf("expected at least one reused-marked row after toggling edited on; got %s", after)
	}
	if !strings.Contains(after, "trow edited") {
		t.Errorf("expected at least one edited-marked row after toggling edited on; got %s", after)
	}
	if !strings.Contains(after, "Reset") {
		t.Errorf("expected the toggle label to read Reset once edited; got %s", after)
	}
}
