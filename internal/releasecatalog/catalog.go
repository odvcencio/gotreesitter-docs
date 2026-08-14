// Package releasecatalog exposes the GoTreeSitter changelog as structured data.
package releasecatalog

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	// SourceCommit is the gotreesitter release commit that supplied CHANGELOG.md.
	SourceCommit = "9fd59953f9f3e22b2d07290138541bc6dd801cf2"

	// LatestReleasedVersion is the newest immutable release in the snapshot.
	LatestReleasedVersion = "v0.50.0"

	// SourceURL links to the exact source used to build this catalog.
	SourceURL = "https://github.com/odvcencio/gotreesitter/blob/" + SourceCommit + "/CHANGELOG.md"

	// SourceSHA256 authenticates the embedded changelog bytes.
	SourceSHA256 = "6db71988ff9556f4448a154c19c61e7bd185959770c724ff6460a30862885ac7"
)

//go:embed CHANGELOG.md
var sourceMarkdown []byte

var (
	releaseHeadingPattern = regexp.MustCompile(`^## \[([^\]]+)\](?: - ([0-9]{4}-[0-9]{2}-[0-9]{2}))?$`)
	versionPattern        = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)
)

// Status identifies whether a release is immutable or still in development.
type Status string

const (
	StatusReleased   Status = "released"
	StatusUnreleased Status = "unreleased"
)

// Catalog contains every release in the pinned changelog snapshot.
type Catalog struct {
	Source   Source
	Releases []Release
}

// Source identifies the upstream changelog snapshot.
type Source struct {
	Commit         string
	URL            string
	SHA256         string
	LatestReleased string
}

// Release contains one version or the current unreleased section.
type Release struct {
	Version         string
	Tag             string
	Date            string
	Status          Status
	SourceLine      int
	SummaryMarkdown string
	Sections        []Section
}

// Section groups changelog entries under one upstream category.
type Section struct {
	Name                 string
	SourceLine           int
	IntroductionMarkdown string
	Entries              []Entry
}

// Entry contains one top-level changelog item.
type Entry struct {
	Markdown     string
	BodyMarkdown string
	PlainText    string
	SourceLine   int
}

// Filter selects changelog entries without changing their release order.
type Filter struct {
	Query    string
	Category string
	Status   Status
}

// Load parses the embedded authoritative snapshot.
func Load() (Catalog, error) {
	sum := sha256.Sum256(sourceMarkdown)
	if actual := hex.EncodeToString(sum[:]); actual != SourceSHA256 {
		return Catalog{}, fmt.Errorf("snapshot SHA-256 is %s, metadata says %s", actual, SourceSHA256)
	}
	releases, err := Parse(sourceMarkdown)
	if err != nil {
		return Catalog{}, err
	}
	catalog := Catalog{
		Source: Source{
			Commit:         SourceCommit,
			URL:            SourceURL,
			SHA256:         SourceSHA256,
			LatestReleased: LatestReleasedVersion,
		},
		Releases: releases,
	}
	if err := validateSource(catalog); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

// Categories returns each upstream category once in alphabetical order.
func (c Catalog) Categories() []string {
	seen := make(map[string]struct{})
	for _, release := range c.Releases {
		for _, section := range release.Sections {
			seen[section.Name] = struct{}{}
		}
	}
	categories := make([]string, 0, len(seen))
	for name := range seen {
		categories = append(categories, name)
	}
	sort.Strings(categories)
	return categories
}

// Filter returns releases that contain at least one selected item.
func (c Catalog) Filter(filter Filter) []Release {
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	category := strings.ToLower(strings.TrimSpace(filter.Category))

	filtered := make([]Release, 0, len(c.Releases))
	for _, release := range c.Releases {
		if filter.Status != "" && release.Status != filter.Status {
			continue
		}

		releaseMatches := query == "" ||
			strings.Contains(strings.ToLower(release.Version), query) ||
			strings.Contains(strings.ToLower(release.Tag), query) ||
			strings.Contains(strings.ToLower(release.SummaryMarkdown), query)

		selected := release
		selected.Sections = nil
		for _, section := range release.Sections {
			if category != "" && strings.ToLower(section.Name) != category {
				continue
			}
			sectionMatches := releaseMatches ||
				strings.Contains(strings.ToLower(section.Name), query) ||
				strings.Contains(strings.ToLower(section.IntroductionMarkdown), query)

			selectedSection := section
			selectedSection.Entries = nil
			for _, entry := range section.Entries {
				if sectionMatches || strings.Contains(strings.ToLower(entry.PlainText), query) {
					selectedSection.Entries = append(selectedSection.Entries, entry)
				}
			}
			if len(selectedSection.Entries) > 0 ||
				(sectionMatches && selectedSection.IntroductionMarkdown != "") {
				selected.Sections = append(selected.Sections, selectedSection)
			}
		}
		if len(selected.Sections) > 0 {
			filtered = append(filtered, selected)
		}
	}
	return filtered
}

// Parse converts a Keep a Changelog-style document into release data.
func Parse(markdown []byte) ([]Release, error) {
	scanner := bufio.NewScanner(bytes.NewReader(markdown))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var (
		releases        []Release
		currentRelease  *Release
		currentSection  *Section
		entryLines      []string
		summaryLines    []string
		introLines      []string
		seenVersions    = make(map[string]struct{})
		lineNumber      int
		entrySourceLine int
	)

	flushEntry := func() {
		if currentSection == nil || len(entryLines) == 0 {
			entryLines = nil
			return
		}
		markdownText := strings.TrimSpace(strings.Join(entryLines, "\n"))
		if markdownText != "" {
			currentSection.Entries = append(currentSection.Entries, Entry{
				Markdown:     markdownText,
				BodyMarkdown: entryBodyMarkdown(markdownText),
				PlainText:    markdownPlainText(markdownText),
				SourceLine:   entrySourceLine,
			})
		}
		entryLines = nil
		entrySourceLine = 0
	}
	flushSectionText := func() {
		if currentSection == nil {
			introLines = nil
			return
		}
		currentSection.IntroductionMarkdown = strings.TrimSpace(strings.Join(introLines, "\n"))
		introLines = nil
	}
	flushReleaseText := func() {
		if currentRelease == nil {
			summaryLines = nil
			return
		}
		currentRelease.SummaryMarkdown = strings.TrimSpace(strings.Join(summaryLines, "\n"))
		summaryLines = nil
	}

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSuffix(scanner.Text(), "\r")

		if match := releaseHeadingPattern.FindStringSubmatch(line); match != nil {
			flushEntry()
			flushSectionText()
			flushReleaseText()

			version := match[1]
			if _, exists := seenVersions[version]; exists {
				return nil, fmt.Errorf("line %d: duplicate release %q", lineNumber, version)
			}
			release, err := parseReleaseHeading(version, match[2], lineNumber)
			if err != nil {
				return nil, err
			}
			releases = append(releases, release)
			currentRelease = &releases[len(releases)-1]
			currentRelease.SourceLine = lineNumber
			currentSection = nil
			seenVersions[version] = struct{}{}
			continue
		}

		if strings.HasPrefix(line, "## ") {
			return nil, fmt.Errorf("line %d: invalid release heading %q", lineNumber, line)
		}
		if currentRelease == nil {
			continue
		}

		if strings.HasPrefix(line, "### ") {
			flushEntry()
			flushSectionText()
			flushReleaseText()

			name := strings.TrimSpace(strings.TrimPrefix(line, "### "))
			if name == "" {
				return nil, fmt.Errorf("line %d: empty category heading", lineNumber)
			}
			currentRelease.Sections = append(currentRelease.Sections, Section{
				Name:       name,
				SourceLine: lineNumber,
			})
			currentSection = &currentRelease.Sections[len(currentRelease.Sections)-1]
			continue
		}
		if strings.HasPrefix(line, "###") {
			return nil, fmt.Errorf("line %d: invalid category heading %q", lineNumber, line)
		}

		if isReferenceDefinition(line) {
			flushEntry()
			continue
		}
		if currentSection == nil {
			summaryLines = append(summaryLines, line)
			continue
		}
		if strings.HasPrefix(line, "- ") {
			flushEntry()
			flushSectionText()
			entrySourceLine = lineNumber
			entryLines = append(entryLines, line)
			continue
		}
		if len(entryLines) > 0 {
			entryLines = append(entryLines, line)
			continue
		}
		introLines = append(introLines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read changelog: %w", err)
	}

	flushEntry()
	flushSectionText()
	flushReleaseText()
	if err := validateReleases(releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func parseReleaseHeading(version, date string, lineNumber int) (Release, error) {
	if version == "Unreleased" {
		if date != "" {
			return Release{}, fmt.Errorf("line %d: Unreleased must not have a date", lineNumber)
		}
		return Release{Version: version, Status: StatusUnreleased}, nil
	}
	if !versionPattern.MatchString(version) {
		return Release{}, fmt.Errorf("line %d: invalid semantic version %q", lineNumber, version)
	}
	if date == "" {
		return Release{}, fmt.Errorf("line %d: released version %q has no date", lineNumber, version)
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return Release{}, fmt.Errorf("line %d: invalid release date %q: %w", lineNumber, date, err)
	}
	return Release{
		Version: version,
		Tag:     "v" + version,
		Date:    date,
		Status:  StatusReleased,
	}, nil
}

func validateReleases(releases []Release) error {
	if len(releases) < 2 {
		return fmt.Errorf("changelog must contain Unreleased and at least one release")
	}
	if releases[0].Status != StatusUnreleased {
		return fmt.Errorf("first changelog section must be Unreleased")
	}
	for _, release := range releases {
		if release.Status != StatusUnreleased && len(release.Sections) == 0 && release.SummaryMarkdown == "" {
			return fmt.Errorf("release %q has no content", release.Version)
		}
		for _, section := range release.Sections {
			if len(section.Entries) == 0 && section.IntroductionMarkdown == "" {
				return fmt.Errorf("release %q category %q has no content", release.Version, section.Name)
			}
		}
	}
	return nil
}

func validateSource(catalog Catalog) error {
	var latestReleased string
	for _, release := range catalog.Releases {
		if release.Status == StatusReleased {
			latestReleased = release.Tag
			break
		}
	}
	if latestReleased == "" {
		return fmt.Errorf("snapshot has no released version")
	}
	if latestReleased != catalog.Source.LatestReleased {
		return fmt.Errorf(
			"snapshot latest release is %s, metadata says %s",
			latestReleased,
			catalog.Source.LatestReleased,
		)
	}
	return nil
}

func isReferenceDefinition(line string) bool {
	line = strings.TrimSpace(line)
	close := strings.Index(line, "]: ")
	return strings.HasPrefix(line, "[") && close > 1
}

func markdownPlainText(markdown string) string {
	var out strings.Builder
	out.Grow(len(markdown))

	for _, r := range markdown {
		switch r {
		case '`', '*', '_', '[', ']', '(', ')', '#':
			out.WriteRune(' ')
		default:
			out.WriteRune(r)
		}
	}
	return strings.Join(strings.FieldsFunc(out.String(), unicode.IsSpace), " ")
}

func entryBodyMarkdown(markdown string) string {
	lines := strings.Split(markdown, "\n")
	if len(lines) == 0 {
		return ""
	}
	lines[0] = strings.TrimPrefix(lines[0], "- ")
	for i := 1; i < len(lines); i++ {
		lines[i] = strings.TrimPrefix(lines[i], "  ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
