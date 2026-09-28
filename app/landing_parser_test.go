package docs

import (
	"strings"
	"testing"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func TestLandingParserExamplesComeFromRealParses(t *testing.T) {
	examples, err := buildLandingParseExamples()
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) != 6 {
		t.Fatalf("got %d parse examples, want 6", len(examples))
	}

	var goExample *landingParseExample
	for i := range examples {
		example := &examples[i]
		if example.SExpr == "" || len(example.Nodes) == 0 || len(example.Tokens) == 0 {
			t.Errorf("%s example is missing parse output", example.Name)
		}
		for _, token := range example.Tokens {
			if token.Start >= token.End || int(token.End) > len(example.Source) {
				t.Errorf("%s token has invalid byte range [%d, %d)", example.Name, token.Start, token.End)
				continue
			}
			if got := example.Source[token.Start:token.End]; got != token.Text {
				t.Errorf("%s token range [%d, %d) contains %q, want %q", example.Name, token.Start, token.End, got, token.Text)
			}
		}
		if example.Name == "go" {
			goExample = example
		}
	}
	if goExample == nil {
		t.Fatal("Go parse example is missing")
	}
	if !strings.Contains(goExample.SExpr, "(function_declaration") {
		t.Fatalf("Go S-expression does not contain function_declaration: %s", goExample.SExpr)
	}

	entry := grammars.DetectLanguageByName("go")
	if entry == nil || entry.Language() == nil {
		t.Fatal("Go grammar is unavailable")
	}
	language := entry.Language()
	tree, err := gts.NewParser(language).Parse([]byte(goExample.Source))
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Release()
	if got, want := goExample.SExpr, tree.RootNode().SExpr(language); got != want {
		t.Fatalf("cached Go S-expression differs from a fresh real parse\n got: %s\nwant: %s", got, want)
	}

	markup, err := landingParserDemoMarkup("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(markup, ".parser-source-token[data-node-id=\"go-node-") || !strings.Contains(markup, "function_declaration") {
		t.Fatal("rendered demo does not connect source tokens to named parse-tree rows")
	}
}
