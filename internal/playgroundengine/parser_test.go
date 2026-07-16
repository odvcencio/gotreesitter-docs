package playgroundengine

import "testing"

func TestParseBuildsTreeAndQueryCaptures(t *testing.T) {
	result := Parse(
		"package main\nfunc alpha() {}\nfunc beta() {}",
		"(function_declaration name: (identifier) @function)",
		"go",
		false,
	)
	if result.ParseError != "" || result.QueryError != "" {
		t.Fatalf("parse errors: parse=%q query=%q", result.ParseError, result.QueryError)
	}
	if result.NodeCount == 0 || len(result.TreeRows) == 0 {
		t.Fatal("expected syntax tree rows")
	}
	if len(result.Captures) != 2 {
		t.Fatalf("captures = %d, want 2", len(result.Captures))
	}
}

func TestParseRejectsOversizedSource(t *testing.T) {
	source := make([]byte, MaxSourceBytes+1)
	if result := Parse(string(source), "", "go", false); result.ParseError == "" {
		t.Fatal("expected oversized source error")
	}
}
