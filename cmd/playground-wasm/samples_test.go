package main

import (
	"testing"

	"github.com/odvcencio/gotreesitter-docs/internal/playgroundengine"
)

// TestLanguageSamplesParseCleanly runs every starter sample through the real
// playground engine. A sample that does not parse, or a query that does not
// compile against its grammar, would greet a visitor with an error box the
// moment they switch languages — so it fails here instead.
func TestLanguageSamplesParseCleanly(t *testing.T) {
	if len(languageSamples) == 0 {
		t.Fatal("no language samples defined")
	}

	for language, sample := range languageSamples {
		t.Run(language, func(t *testing.T) {
			result := playgroundengine.Parse(sample.Source, sample.Query, language, false)

			if result.ParseError != "" {
				t.Fatalf("sample does not parse: %s", result.ParseError)
			}
			if result.QueryError != "" {
				t.Fatalf("query does not compile: %s", result.QueryError)
			}
			if result.HasErrors {
				t.Errorf("sample parses with ERROR nodes in the tree")
			}
			if result.NodeCount == 0 || len(result.TreeRows) == 0 {
				t.Fatal("sample produced an empty tree")
			}
			// A query that matches nothing teaches the visitor nothing.
			if len(result.Captures) == 0 {
				t.Errorf("query produced no captures against its own sample")
			}
		})
	}
}

func TestSampleForKnownAndUnknownLanguages(t *testing.T) {
	if _, ok := sampleFor("go"); !ok {
		t.Error("expected a Go sample")
	}
	if _, ok := sampleFor("not-a-language"); ok {
		t.Error("expected no sample for an unknown language")
	}
}
