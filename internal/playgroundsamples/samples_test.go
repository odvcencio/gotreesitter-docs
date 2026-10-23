package playgroundsamples

import (
	"strings"
	"testing"

	"github.com/odvcencio/gotreesitter-docs/internal/playgroundengine"
)

func TestNamedAddSamplesParseWithTheirQueries(t *testing.T) {
	for _, language := range []string{"go", "python", "rust", "typescript", "json", "html"} {
		sample, ok := Named(SampleAdd, language)
		if !ok {
			t.Fatalf("no %q sample for %s", SampleAdd, language)
		}
		result := playgroundengine.Parse(sample.Source, sample.Query, language, false)
		if result.ParseError != "" || result.QueryError != "" || result.HasErrors || len(result.Captures) == 0 {
			t.Errorf("%s/%s: parse=%q query=%q errors=%v captures=%d", SampleAdd, language, result.ParseError, result.QueryError, result.HasErrors, len(result.Captures))
		}
	}
	if _, ok := Named(SampleAdd, "lua"); ok {
		t.Error("add sample must not exist for lua")
	}
	if sample, ok := Named("not-a-sample", "go"); !ok || !strings.Contains(sample.Source, "package main") {
		t.Error("an unknown sample name falls back to the greet starter")
	}
}
