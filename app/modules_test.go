package docs

import (
	"testing"

	"m31labs.dev/gosx/server"
)

func TestMergeDocsMetadataPreservesStructuredSocialMetadata(t *testing.T) {
	extra := server.Metadata{
		MetadataBase: "https://example.com",
		Alternates:   &server.Alternates{Canonical: "https://example.com/changelog"},
		OpenGraph:    &server.OpenGraph{Title: "Changelog"},
		Twitter:      &server.Twitter{Card: "summary_large_image"},
		JSONLD:       []any{map[string]any{"@type": "WebPage"}},
	}
	merged := mergeDocsMetadata(server.Metadata{
		Title:       server.Title{Default: "Default"},
		Description: "Default description",
	}, extra)

	if merged.MetadataBase != extra.MetadataBase ||
		merged.Alternates != extra.Alternates ||
		merged.OpenGraph != extra.OpenGraph ||
		merged.Twitter != extra.Twitter ||
		len(merged.JSONLD) != 1 {
		t.Fatalf("structured metadata was not preserved: %#v", merged)
	}
	if merged.Title.Default != "Default" || merged.Description != "Default description" {
		t.Fatalf("base metadata changed unexpectedly: %#v", merged)
	}
}
