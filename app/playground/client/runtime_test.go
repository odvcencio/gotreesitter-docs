package main

import (
	"testing"
	"unicode/utf16"

	"github.com/odvcencio/gotreesitter/grammars"
)

func TestRuntimeDocumentKeepsOneIncrementalParserTree(t *testing.T) {
	language := &runtimeLanguage{name: "json", language: grammars.JsonLanguage()}
	document := runtimeDocument{}
	if err := document.open(language, `{"name":"gotreesitter","version":36}`); err != nil {
		t.Fatal(err)
	}
	firstTree := document.tree
	if firstTree == nil || document.parser == nil || document.revision != 1 {
		t.Fatalf("open state: tree=%p parser=%p revision=%d", firstTree, document.parser, document.revision)
	}
	if err := document.update(`{"name":"gotreesitter","version":37}`); err != nil {
		t.Fatal(err)
	}
	if document.tree == nil || document.revision != 2 {
		t.Fatalf("update state: tree=%p revision=%d", document.tree, document.revision)
	}
	if got := string(document.tree.Source()); got != `{"name":"gotreesitter","version":37}` {
		t.Fatalf("tree source = %q", got)
	}
	if root := buildViewTree(document.tree, language.language, maxTreeNodes); root == nil || root.typ != "document" {
		t.Fatalf("view root = %#v", root)
	}
	query := queryDocument(&document, `(string) @value`)
	if query.Err != nil || len(query.captures) == 0 {
		t.Fatalf("query result: err=%v captures=%d", query.Err, len(query.captures))
	}
	document.close()
	if document.tree != nil || document.parser != nil || document.language != nil {
		t.Fatalf("close retained runtime state: %#v", document)
	}
}

func TestUTF16EditBetweenWidensSurrogateBoundary(t *testing.T) {
	oldSource := utf16.Encode([]rune("a😀b"))
	newSource := utf16.Encode([]rune("a😺b"))
	edit, changed := utf16EditBetween(oldSource, newSource)
	if !changed {
		t.Fatal("replacement was not detected")
	}
	if edit.StartCodeUnit != 1 || edit.OldEndCodeUnit != 3 || edit.NewEndCodeUnit != 3 {
		t.Fatalf("surrogate edit = %#v", edit)
	}
}

func TestHeuristicDetectPreservesExistingSignatures(t *testing.T) {
	tests := map[string]string{
		"#!/usr/bin/env python3\nprint('hi')\n":             "python",
		"package main\nfunc main() {}\n":                    "go",
		"#include <stdio.h>\nint main(void) {}\n":           "c",
		"{\n  \"name\": \"gotreesitter\"\n}\n":              "json",
		"fn main() { let mut value = thing::new(); }\n":     "rust",
		"# gotreesitter\n\n```go\npackage main\n```\n":      "markdown",
		"SELECT name FROM grammars WHERE enabled = true;\n": "sql",
	}
	for source, want := range tests {
		if got := heuristicDetect(source); got != want {
			t.Errorf("heuristicDetect(%q) = %q, want %q", source, got, want)
		}
	}
}

func TestTokenAndCapturePresentationRemainDeterministic(t *testing.T) {
	if got := tokenClass("function.method"); got != "tk-fn" {
		t.Fatalf("token class = %q", got)
	}
	if first, second := captureColor("definition.function"), captureColor("definition.function"); first != second || first < 0 || first > 7 {
		t.Fatalf("capture colors = %d, %d", first, second)
	}
}

func TestRuntimeDocumentRunsPredicateQueryAfterIncrementalUpdate(t *testing.T) {
	language := &runtimeLanguage{name: "python", language: grammars.PythonLanguage()}
	document := runtimeDocument{}
	if err := document.open(language, "def fib(n):\n    return n\n"); err != nil {
		t.Fatal(err)
	}
	defer document.close()
	if err := document.update(languageSourceForQueryTest); err != nil {
		t.Fatal(err)
	}
	result := queryDocument(&document, `((identifier) @id (#match? @id "^f"))`)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if len(result.captures) == 0 || result.captures[0].name != "id" {
		t.Fatalf("predicate captures = %#v", result.captures)
	}
}

func TestRuntimeLanguageHighlightsPythonUTF16Document(t *testing.T) {
	entry := grammars.DetectLanguageByName("python")
	if entry == nil {
		t.Fatal("python language is not registered")
	}
	language, err := loadRuntimeLanguage(entry.Name, grammars.BlobByName(entry.Name), entry.HighlightQuery)
	if err != nil {
		t.Fatal(err)
	}
	if language.highlightErr != nil {
		t.Fatal(language.highlightErr)
	}
	document := runtimeDocument{}
	if err := document.open(language, "def fib(n):\n    return n\n"); err != nil {
		t.Fatal(err)
	}
	defer document.close()
	if ranges := documentTokenRanges(&document); len(ranges) == 0 {
		t.Fatal("python highlight query produced no ranges")
	}
}

const languageSourceForQueryTest = "def fib(n):\n    first = n\n    return first\n"
