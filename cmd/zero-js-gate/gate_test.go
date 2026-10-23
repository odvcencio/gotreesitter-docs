package main

import (
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// walkFiles lists every file under root as a slash-separated relative path,
// standing in for the git file lister the command uses.
func walkFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func fixtureViolations(t *testing.T) []Violation {
	t.Helper()
	root := filepath.Join("testdata", "repo")
	violations, err := Scan(root, InScope(walkFiles(t, root)))
	if err != nil {
		t.Fatal(err)
	}
	return violations
}

func TestGateFindsEveryRuleAndRespectsScope(t *testing.T) {
	want := map[string]int{
		"app/bad.gsx:script-tag":                  1,
		"app/bad.gsx:inline-handler":              1,
		"app/widget.go:script-tag":                1,
		"app/widget.go:script-element":            1,
		"public/vendor.js:authored-script-file":   1,
		"cmd/demo-wasm/main_js.go:syscall-js-dom": 1,
		"cmd/demo-wasm/boot.go:js-in-go-string":   1,
		"main.go:script-element":                  1,
	}
	got := map[string]int{}
	for _, v := range fixtureViolations(t) {
		got[v.Path+":"+v.Rule]++
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("violations = %v, want %v", got, want)
	}
}

func TestAllowlistMatchesAndFlagsStaleEntries(t *testing.T) {
	allow, err := ParseAllowlist(strings.NewReader("# comment\nmain.go\tscript-element\tbeacon\tthird-party\napp/gone.go\tscript-tag\told\tG99\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(allow) != 2 {
		t.Fatalf("parsed %d entries, want 2", len(allow))
	}
	violations := fixtureViolations(t)
	remaining, stale := Apply(violations, allow)
	for _, v := range remaining {
		if v.Path == "main.go" {
			t.Errorf("main.go violation %v should be allowlisted", v)
		}
	}
	if len(remaining) != len(violations)-1 {
		t.Errorf("remaining = %d, want %d", len(remaining), len(violations)-1)
	}
	if len(stale) != 1 || stale[0].Path != "app/gone.go" {
		t.Fatalf("stale = %+v, want only app/gone.go", stale)
	}
}

func TestParseAllowlistRejectsMalformedLines(t *testing.T) {
	if _, err := ParseAllowlist(strings.NewReader("main.go\tscript-element\tonly three columns\n")); err == nil {
		t.Fatal("want an error for a line without four columns")
	}
}

func TestInScopeCoversTheGuardedPathsOnly(t *testing.T) {
	got := InScope([]string{
		"app/page.gsx", "public/docs.css", "main.go", "cmd/playground-wasm/main_js.go",
		"cmd/verify-playground-browser/main.go", "scripts/tool.js", "internal/x/y.go", "assets.go",
	})
	want := []string{"app/page.gsx", "cmd/playground-wasm/main_js.go", "main.go", "public/docs.css"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InScope = %v, want %v", got, want)
	}
}
