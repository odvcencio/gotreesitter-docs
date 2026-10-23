package docs

import (
	"testing"

	"github.com/odvcencio/gotreesitter/grammars"
)

func TestReleaseDateComesFromTheVendoredChangelog(t *testing.T) {
	if got := releaseDateFor("v0.56.0"); got != "2026-10-13" {
		t.Fatalf("release date for v0.56.0 = %q", got)
	}
	if got := releaseDateFor("v9.9.9"); got != "" {
		t.Fatalf("release date for an unknown version = %q, want empty", got)
	}
}

// The expected numbers come from gotreesitter v0.56.0 docs/languages.md:3-6.
// A mismatch means the doc or the registry moved, so the home page must not
// claim either number blindly.
func TestPinnedRegistryFactsMatchTheLanguageGuide(t *testing.T) {
	facts := PinnedRegistryFacts()
	if facts["grammars"] != 206 {
		t.Errorf("grammars = %v, want 206", facts["grammars"])
	}
	if facts["tokenSources"] != 7 {
		t.Errorf("tokenSources = %v, want 7", facts["tokenSources"])
	}
	// The default registry wires five of the seven hand-written token sources
	// (go and lua stay opt-in). If this count moves, re-read docs/languages.md.
	wired := 0
	for _, entry := range grammars.AllLanguages() {
		if entry.TokenSourceFactory != nil {
			wired++
		}
	}
	if wired != 5 {
		t.Errorf("registry wires %d token sources, expected 5; re-check handWrittenTokenSourceCount", wired)
	}
}
