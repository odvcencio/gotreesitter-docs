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

func TestCanonicalMetadataUsesProductionOrigin(t *testing.T) {
	for _, path := range []string{"changelog", "/changelog"} {
		meta := CanonicalMetadata(path)
		if meta.MetadataBase != SiteURL {
			t.Fatalf("metadata base = %q, want %q", meta.MetadataBase, SiteURL)
		}
		if meta.Alternates == nil || meta.Alternates.Canonical != SiteURL+"/changelog" {
			t.Fatalf("canonical metadata for %q = %#v", path, meta.Alternates)
		}
	}
}
