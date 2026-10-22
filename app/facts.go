package docs

import (
	"github.com/odvcencio/gotreesitter-docs/internal/releasecatalog"
	"github.com/odvcencio/gotreesitter/grammars"
)

// externalScannerCount is not derivable from grammars.LangEntry (it has no
// scanner field). Source: gotreesitter v0.56.0 docs/languages.md lines 5-6,
// read 2026-10-22. Re-check on every gotreesitter bump.
const externalScannerCount = 119

// handWrittenTokenSourceCount is the upstream guide's count of hand-written Go
// token sources (authzed, c, cpp, go, java, json, lua; docs/languages.md line
// 6, v0.56.0, read 2026-10-22). The linked registry wires only five of them by
// default (go and lua stay opt-in), so counting TokenSourceFactory fields would
// understate the guide. Re-check on every gotreesitter bump.
const handWrittenTokenSourceCount = 7

// PlaygroundGTSReleaseDate returns the pinned release's date from the vendored
// changelog snapshot, or "" for a dev build.
func PlaygroundGTSReleaseDate() string { return releaseDateFor(PlaygroundGTSVersion()) }

func releaseDateFor(version string) string {
	catalog, err := releasecatalog.Load()
	if err != nil {
		return ""
	}
	for _, release := range catalog.Releases {
		if release.Tag == version {
			return release.Date
		}
	}
	return ""
}

// PinnedRegistryFacts counts the grammars the server linked and carries the
// upstream guide's scanner and token-source counts.
func PinnedRegistryFacts() map[string]any {
	entries := grammars.AllLanguages()
	return map[string]any{
		"grammars":     len(entries),
		"tokenSources": handWrittenTokenSourceCount,
		"scanners":     externalScannerCount,
	}
}
