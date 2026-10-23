package docs

import (
	"strings"
	"testing"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter-docs/internal/playgroundsamples"
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
	if !strings.Contains(markup, `.parser-tree-row[data-node-id="go-node-0"]:hover) .parser-source-token[data-node-id="go-node-0"]`) {
		t.Fatal("rendered demo does not connect tree rows back to their source tokens")
	}
	if !strings.Contains(markup, `<li class="parser-tree-row" data-node-id="go-node-0" data-depth="0" tabindex="0"`) {
		t.Fatal("tree rows are not keyboard focusable")
	}
}

func TestHeroSnippetsAreTheAddSamples(t *testing.T) {
	examples, err := buildLandingParseExamples()
	if err != nil {
		t.Fatal(err)
	}
	for _, example := range examples {
		sample, ok := playgroundsamples.Named(playgroundsamples.SampleAdd, example.Name)
		if !ok || example.Source != sample.Source {
			t.Errorf("%s hero source differs from the add sample", example.Name)
		}
	}
	markup, err := landingParserDemoMarkup("")
	if err != nil {
		t.Fatal(err)
	}
	for _, example := range examples {
		want := `<a class="parser-open" href="/playground?lang=` + example.Name + `&amp;sample=add" data-gosx-link="true">`
		if !strings.Contains(markup, want) {
			t.Errorf("%s section lacks its playground link %q", example.Name, want)
		}
	}
}
