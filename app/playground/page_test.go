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
		`<Surface`,
		`name="GoTreesitterPlayground"`,
		`runtime="go-wasm"`,
		`wasmPath={data.wasmURL}`,
		`requiredCapabilities="wasm fetch"`,
		`role="tree"`,
		`aria-label="Syntax tree"`,
	)
	assertFileContainsAll(t, filepath.Join(root, "app", "playground", "client", "render_js.go"),
		`row.Call("setAttribute", "role", "treeitem")`,
		`row.Call("setAttribute", "aria-level"`,
		`row.Call("setAttribute", "aria-selected", "false")`,
		`row.Set("tabIndex", -1)`,
		`case "ArrowDown":`,
		`case "ArrowUp":`,
		`case "Home":`,
		`case "End":`,
		`case "Enter", " ":`,
		`p.activateTreeRow(index)`,
	)
	assertFileContainsAll(t, filepath.Join(root, "public", "playground", "playground.css"),
		`.pg-tline:focus-visible`,
	)
}

func TestPlaygroundHasNoAuthoredJavaScriptEscapeHatch(t *testing.T) {
	root := playgroundAppRoot()
	var clientSources []string
	for _, directory := range []string{filepath.Join(root, "app"), filepath.Join(root, "public")} {
		if err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".js", ".mjs", ".ts", ".tsx":
				clientSources = append(clientSources, path)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if len(clientSources) != 0 {
		t.Fatalf("application client scripts still exist: %v", clientSources)
	}
	assertFileContainsAll(t, filepath.Join(root, "app", "playground", "client", "main_js.go"),
		`enginewasm.Register(componentName, mountPlayground)`,
		`enginewasm.HandleFunc(p.dispose)`,
		`p.document.close()`,
	)
}

func TestPlaygroundLegendConsumesItsDocumentFragmentOnce(t *testing.T) {
	root := playgroundAppRoot()
	path := filepath.Join(root, "app", "playground", "client", "render_js.go")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const consume = `p.ui.legend.Call("replaceChildren", fragment)`
	if count := strings.Count(string(contents), consume); count != 1 {
		t.Fatalf("legend fragment consumption count = %d, want 1", count)
	}
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
