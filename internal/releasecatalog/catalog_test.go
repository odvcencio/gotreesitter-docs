package releasecatalog

import (
	"strings"
	"testing"
)

func TestLoadDistinguishesReleaseFromEmptyUnreleased(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Source.Commit != SourceCommit {
		t.Fatalf("source commit = %q, want %q", catalog.Source.Commit, SourceCommit)
	}
	// Derive from the catalog constant rather than pinning a version, so
	// refreshing the snapshot to a new release does not fail this test for a
	// reason unrelated to what it checks.
	if catalog.Source.LatestReleased != LatestReleasedVersion {
		t.Fatalf("latest released = %q, want %q", catalog.Source.LatestReleased, LatestReleasedVersion)
	}
	if len(catalog.Releases) < 70 {
		t.Fatalf("release count = %d, want at least 70", len(catalog.Releases))
	}

	unreleased := catalog.Releases[0]
	if unreleased.Version != "Unreleased" || unreleased.Status != StatusUnreleased {
		t.Fatalf("first release = %#v, want Unreleased", unreleased)
	}
	if unreleased.Date != "" || unreleased.Tag != "" {
		t.Fatalf("Unreleased has immutable metadata: %#v", unreleased)
	}
	if unreleased.SummaryMarkdown != "" || len(unreleased.Sections) != 0 {
		t.Fatalf("Unreleased = %#v, want no entries", unreleased)
	}

	// The newest immutable release: assert its shape, not its identity. Pinning
	// a version here made every snapshot refresh fail for a reason unrelated to
	// the parsing this test covers.
	released := catalog.Releases[1]
	if released.Tag != LatestReleasedVersion || released.Status != StatusReleased {
		t.Fatalf("latest immutable release = %#v, want %s released", released, LatestReleasedVersion)
	}
	if released.Date == "" {
		t.Fatalf("latest immutable release has no date: %#v", released)
	}
	if len(released.Sections) == 0 || len(released.Sections[0].Entries) == 0 {
		t.Fatalf("latest immutable release has no entries: %#v", released)
	}

	// Content assertions name their release explicitly, so they keep testing
	// what they were written to test as newer releases land above them.
	v0500 := releaseByTag(t, catalog, "v0.50.0")
	if !releaseContains(v0500, "TestOutlineOracleDifferential") {
		t.Fatal("v0.50.0 does not contain the outline oracle differential")
	}
	if !releaseContains(v0500, "signed right-shift operator") {
		t.Fatal("v0.50.0 does not contain the TypeScript shift-operator fix")
	}
	if released.SourceLine == 0 || released.Sections[0].SourceLine == 0 ||
		released.Sections[0].Entries[0].SourceLine == 0 {
		t.Fatalf("released evidence has no source lines: %#v", released)
	}
}

func TestParsePreservesNestedEntryMarkdown(t *testing.T) {
	input := []byte(`# Changelog

## [Unreleased]

### Performance

- The parser reuses work.

  - Allocation falls.
  - Time stays unchanged.

## [0.1.0] - 2026-02-19

### Added

- Initial release.

[Unreleased]: https://example.com/compare
[0.1.0]: https://example.com/tag
`)
	releases, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(releases[0].Sections[0].Entries); got != 1 {
		t.Fatalf("entry count = %d, want 1", got)
	}
	entry := releases[0].Sections[0].Entries[0]
	if !strings.Contains(entry.Markdown, "  - Allocation falls.") {
		t.Fatalf("nested list was not preserved:\n%s", entry.Markdown)
	}
	if !strings.Contains(entry.BodyMarkdown, "- Allocation falls.") ||
		strings.HasPrefix(entry.BodyMarkdown, "- The parser") {
		t.Fatalf("entry body was not unwrapped:\n%s", entry.BodyMarkdown)
	}
	if strings.Contains(entry.Markdown, "[Unreleased]:") {
		t.Fatalf("reference definition leaked into entry:\n%s", entry.Markdown)
	}
	if !strings.Contains(entry.PlainText, "Allocation falls.") {
		t.Fatalf("plain text = %q", entry.PlainText)
	}
}

func TestParseAllowsEmptyUnreleased(t *testing.T) {
	releases, err := Parse([]byte(`# Changelog

## [Unreleased]

## [0.48.0] - 2026-08-01

### Added

- Release.
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("release count = %d, want 2", len(releases))
	}
	unreleased := releases[0]
	if unreleased.Status != StatusUnreleased || unreleased.SummaryMarkdown != "" || len(unreleased.Sections) != 0 {
		t.Fatalf("unreleased = %#v, want empty Unreleased", unreleased)
	}
}

func TestFilterFindsEntriesWithoutMixingReleaseStatus(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	releases := catalog.Filter(Filter{
		Query:  "deferred parent links",
		Status: StatusReleased,
	})
	if len(releases) == 0 {
		t.Fatal("released recovery search returned no results")
	}
	if releases[0].Tag != "v0.47.1" {
		t.Fatalf("first released recovery result = %q, want v0.47.1", releases[0].Tag)
	}
	for _, release := range releases {
		if release.Status != StatusReleased {
			t.Fatalf("released filter returned status %q", release.Status)
		}
	}

	releases = catalog.Filter(Filter{
		Query:    "expected Hurl and INI root types",
		Category: "Fixed",
		Status:   StatusReleased,
	})
	if len(releases) != 1 || releases[0].Tag != "v0.48.0" {
		t.Fatalf("released retirement results = %#v, want v0.48.0 only", releases)
	}
	if len(releases[0].Sections) != 1 || releases[0].Sections[0].Name != "Fixed" {
		t.Fatalf("filtered sections = %#v, want Fixed only", releases[0].Sections)
	}

	releases = catalog.Filter(Filter{Status: StatusUnreleased})
	if len(releases) != 0 {
		t.Fatalf("unreleased results = %#v, want no entries", releases)
	}

	releases = catalog.Filter(Filter{
		Query:  "four inert result-compatibility dispatcher arms",
		Status: StatusReleased,
	})
	if len(releases) != 1 || releases[0].Tag != "v0.48.0" {
		t.Fatalf("four-grammar retirement results = %#v, want v0.48.0 only", releases)
	}
}

func TestCategoriesAreUniqueAndSorted(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	categories := catalog.Categories()
	if len(categories) < 10 {
		t.Fatalf("category count = %d, want at least 10", len(categories))
	}
	for i := 1; i < len(categories); i++ {
		if categories[i-1] >= categories[i] {
			t.Fatalf("categories are not unique and sorted: %q then %q", categories[i-1], categories[i])
		}
	}
}

func TestParseRejectsAmbiguousReleaseMetadata(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "released version without date",
			input: "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- Work.\n\n## [0.1.0]\n\n### Added\n\n- Release.\n",
			want:  "has no date",
		},
		{
			name:  "unreleased with date",
			input: "# Changelog\n\n## [Unreleased] - 2026-07-28\n\n### Added\n\n- Work.\n\n## [0.1.0] - 2026-02-19\n\n### Added\n\n- Release.\n",
			want:  "Unreleased must not have a date",
		},
		{
			name:  "duplicate release",
			input: "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- Work.\n\n## [0.1.0] - 2026-02-19\n\n### Added\n\n- Release.\n\n## [0.1.0] - 2026-02-20\n\n### Fixed\n\n- Again.\n",
			want:  "duplicate release",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(test.input))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want text %q", err, test.want)
			}
		})
	}
}

func releaseContains(release Release, text string) bool {
	for _, section := range release.Sections {
		if strings.Contains(section.IntroductionMarkdown, text) {
			return true
		}
		for _, entry := range section.Entries {
			if strings.Contains(entry.Markdown, text) {
				return true
			}
		}
	}
	return strings.Contains(release.SummaryMarkdown, text)
}


// releaseByTag finds a release by tag so content assertions do not depend on
// slice position, which shifts by one with every release.
func releaseByTag(t *testing.T, catalog Catalog, tag string) Release {
	t.Helper()
	for _, release := range catalog.Releases {
		if release.Tag == tag {
			return release
		}
	}
	t.Fatalf("release %s is not in the catalog", tag)
	return Release{}
}
