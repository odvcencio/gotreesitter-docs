package playgroundengine

import (
	"strings"
	"testing"

	"github.com/odvcencio/gotreesitter/grammars"
)

func TestComputeHighlightGo(t *testing.T) {
	entry := grammars.DetectLanguageByName("go")
	if entry == nil {
		t.Fatal("go grammar not registered")
	}
	lang := entry.Language()
	if lang == nil {
		t.Fatal("go language failed to load")
	}
	source := "package main\n\nfunc greet(name string) string {\n\treturn \"hey, \" + name\n}\n"
	spans, notice := ComputeHighlight(source, "go", lang)
	if notice != "" {
		t.Fatalf("unexpected notice: %q", notice)
	}
	if len(spans) == 0 {
		t.Fatal("expected highlight spans for go source")
	}
	var sawKw, sawStr, sawFn bool
	for _, span := range spans {
		if span.StartByte < 0 || span.EndByte > len(source) || span.EndByte <= span.StartByte {
			t.Fatalf("invalid span range %+v", span)
		}
		switch span.Class {
		case "tk-kw":
			sawKw = true
		case "tk-str":
			sawStr = true
		case "tk-fn":
			sawFn = true
		}
	}
	if !sawKw || !sawStr || !sawFn {
		t.Fatalf("expected tk-kw/tk-str/tk-fn among spans, got kw=%v str=%v fn=%v", sawKw, sawStr, sawFn)
	}
}

func TestComputeHighlightAcrossLanguages(t *testing.T) {
	cases := []struct {
		lang   string
		source string
	}{
		{"json", `{"name": "gopher", "count": 3, "ok": true}`},
		{"python", "def greet(name):\n    return \"hey, \" + name\n"},
		{"javascript", "function greet(name) {\n  return `hey, ${name}`;\n}\n"},
		{"typescript", "function greet(name: string): string {\n  return `hey, ${name}`;\n}\n"},
		{"bash", "#!/usr/bin/env bash\necho \"hello $USER\"\n"},
		{"rust", "fn greet(name: &str) -> String {\n    format!(\"hey, {}\", name)\n}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			entry := grammars.DetectLanguageByName(tc.lang)
			if entry == nil {
				t.Fatalf("%s grammar not registered", tc.lang)
			}
			lang := entry.Language()
			if lang == nil {
				t.Fatalf("%s language failed to load", tc.lang)
			}
			spans, notice := ComputeHighlight(tc.source, tc.lang, lang)
			if len(spans) == 0 {
				t.Fatalf("expected highlight spans for %s (notice=%q)", tc.lang, notice)
			}
		})
	}
}

func TestComputeHighlightRejectsOversizedSource(t *testing.T) {
	entry := grammars.DetectLanguageByName("go")
	lang := entry.Language()
	source := strings.Repeat("a", MaxSourceBytes+1)
	spans, notice := ComputeHighlight(source, "go", lang)
	if spans != nil {
		t.Fatal("expected no spans for oversized source")
	}
	if notice == "" {
		t.Fatal("expected a notice for oversized source")
	}
}

func TestComputeHighlightUnknownLanguage(t *testing.T) {
	entry := grammars.DetectLanguageByName("go")
	lang := entry.Language()
	spans, notice := ComputeHighlight("x", "not-a-real-language", lang)
	if spans != nil {
		t.Fatal("expected no spans for an unresolvable language name")
	}
	if notice == "" {
		t.Fatal("expected a notice explaining the fallback")
	}
}

func TestTkClassForCapture(t *testing.T) {
	cases := map[string]string{
		"keyword":             "tk-kw",
		"keyword.operator":    "tk-op",
		"keyword.return":      "tk-kw",
		"operator":            "tk-op",
		"string":              "tk-str",
		"string.special":      "tk-str",
		"comment":             "tk-cm",
		"number":              "tk-num",
		"type":                "tk-ty",
		"type.builtin":        "tk-ty",
		"function":            "tk-fn",
		"function.builtin":    "tk-fn",
		"punctuation.bracket": "tk-pn",
		"variable":            "",
		"variable.builtin":    "",
		"_type":               "",
		"":                    "",
	}
	for capture, want := range cases {
		if got := tkClassForCapture(capture); got != want {
			t.Errorf("tkClassForCapture(%q) = %q, want %q", capture, got, want)
		}
	}
}
