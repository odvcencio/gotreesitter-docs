package playground

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntaxTreeKeyboardContract(t *testing.T) {
	root := playgroundAppRoot()
	assertFileContainsAll(t, filepath.Join(root, "app", "playground", "page.gsx"),
		`role="tree"`,
		`aria-label="Syntax tree"`,
	)
	assertFileContainsAll(t, filepath.Join(root, "public", "playground", "playground.js"),
		`row.setAttribute("role", "treeitem")`,
		`row.setAttribute("aria-level", String(depth + 1))`,
		`row.setAttribute("aria-selected", "false")`,
		`row.tabIndex = -1`,
		`case "ArrowDown":`,
		`case "ArrowUp":`,
		`case "Home":`,
		`case "End":`,
		`case "Enter":`,
		`case " ":`,
		`activateTreeRow(index)`,
	)
	assertFileContainsAll(t, filepath.Join(root, "public", "playground", "playground.css"),
		`.pg-tline:focus-visible`,
	)
}

func assertFileContainsAll(t *testing.T, path string, snippets ...string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, snippet := range snippets {
		if !strings.Contains(string(contents), snippet) {
			t.Errorf("%s does not contain %q", path, snippet)
		}
	}
}
