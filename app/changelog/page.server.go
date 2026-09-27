package changelog

import (
	"embed"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	"github.com/odvcencio/gotreesitter-docs/internal/releasecatalog"
	"m31labs.dev/gosx"
	"m31labs.dev/gosx/route"
	"m31labs.dev/gosx/server"
)

//go:embed page.gsx
var pageSource embed.FS

const repositoryURL = "https://github.com/odvcencio/gotreesitter"

var (
	catalog, catalogErr = releasecatalog.Load()

	pullRequestPattern = regexp.MustCompile(`(?i)\bPRs?\s+#([0-9]+)(?:\s+and\s+#([0-9]+))?`)
	issuePattern       = regexp.MustCompile(`(?i)\bissue\s+#([0-9]+)`)
	commitPattern      = regexp.MustCompile("`([0-9a-f]{7,40})`")
	tagPattern         = regexp.MustCompile(`\bv[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?\b`)
	// These hashes are explicitly identified in the changelog as commits from
	// grammar repositories, not commits in the gotreesitter repository.
	nonRepositoryCommitIDs = map[string]struct{}{
		"172ada1cc4117d0260d9340680b4134adba2bc2c": {},
		"41d6e5fe811ec94229ee71771174a8cce558dfee": {},
		"48ab75f29abaa315fad7fa7b8338f92bb07376a7": {},
		"5739fd79bcfc75ba7526773d0cf634521f8aca3c": {},
		"587f30d184b058450be2a2330878210c5f33b3f9": {},
		"61a7c75e225e3035390be32d635545e40d8c5faf": {},
	}
)

func init() {
	docsapp.RegisterStaticDocsPage(
		"Changelog",
		"Explore every GoTreeSitter release, current unreleased work, upgrade impact, and source evidence.",
		"/changelog",
		route.FileModuleOptions{
			Load: loadChangelog,
			Metadata: func(ctx *route.RouteContext, page route.FilePage, data any) (server.Metadata, error) {
				return changelogMetadata(), nil
			},
		},
	)
}

func changelogMetadata() server.Metadata {
	const (
		title       = "Changelog | gotreesitter"
		description = "Explore GoTreeSitter releases, current work, upgrade impact, and source evidence."
	)
	image := server.MediaAsset{
		URL:    docsapp.SiteURL + docsapp.PublicAssetURL("social/changelog.png"),
		Width:  1200,
		Height: 630,
		Alt:    "GoTreeSitter Changelog with an abstract syntax-tree release timeline.",
		Type:   "image/png",
	}
	return server.Metadata{
		Title:        server.Title{Absolute: title},
		Description:  description,
		MetadataBase: docsapp.SiteURL,
		Alternates:   &server.Alternates{Canonical: docsapp.SiteURL + "/changelog"},
		OpenGraph: &server.OpenGraph{
			Type:        "website",
			URL:         docsapp.SiteURL + "/changelog",
			SiteName:    "gotreesitter",
			Title:       title,
			Description: description,
			Images:      []server.MediaAsset{image},
		},
		Twitter: &server.Twitter{
			Card:        "summary_large_image",
			Title:       title,
			Description: description,
			Images:      []server.MediaAsset{image},
		},
	}
}

func loadChangelog(ctx *route.RouteContext, _ route.FilePage) (any, error) {
	if catalogErr != nil {
		return nil, fmt.Errorf("load changelog catalog: %w", catalogErr)
	}
	query := strings.TrimSpace(ctx.Query("q"))
	category := strings.TrimSpace(ctx.Query("category"))
	status := normalizeStatus(ctx.Query("status"))

	filtered := catalog.Filter(releasecatalog.Filter{
		Query:    query,
		Category: category,
		Status:   status,
	})
	releases, resultCount, err := buildReleaseViews(filtered)
	if err != nil {
		return nil, err
	}

	totalEntries := catalogEntryCount(catalog.Releases)
	currentEntries := 0
	if len(catalog.Releases) > 0 {
		currentEntries = catalogEntryCount(catalog.Releases[:1])
	}

	return map[string]any{
		"filterForm":       renderFilterForm(query, category, status, catalog.Categories()),
		"hasFilters":       query != "" || category != "" || status != "",
		"query":            query,
		"resultCount":      resultCount,
		"releaseCount":     len(catalog.Releases) - 1,
		"totalEntries":     totalEntries,
		"currentEntries":   currentEntries,
		"latestVersion":    catalog.Source.LatestReleased,
		"earliestVersion":  catalog.Releases[len(catalog.Releases)-1].Tag,
		"earliestDate":     catalog.Releases[len(catalog.Releases)-1].Date,
		"sourceURL":        catalog.Source.URL,
		"sourceCommit":     catalog.Source.Commit[:12],
		"sourceSHA256":     catalog.Source.SHA256,
		"releases":         releases,
		"hasResults":       len(releases) > 0,
		"versionLinks":     buildVersionLinks(),
		"campaignTrail":    campaignTrail(),
		"latestReleaseURL": repositoryURL + "/releases/tag/" + catalog.Source.LatestReleased,
	}, nil
}

func normalizeStatus(value string) releasecatalog.Status {
	switch releasecatalog.Status(strings.ToLower(strings.TrimSpace(value))) {
	case releasecatalog.StatusReleased:
		return releasecatalog.StatusReleased
	case releasecatalog.StatusUnreleased:
		return releasecatalog.StatusUnreleased
	default:
		return ""
	}
}

func buildReleaseViews(releases []releasecatalog.Release) ([]map[string]any, int, error) {
	views := make([]map[string]any, 0, len(releases))
	resultCount := 0
	for _, release := range releases {
		fullIndex := catalogReleaseIndex(release.Version)
		sections := make([]map[string]any, 0, len(release.Sections))
		for _, section := range release.Sections {
			introduction, err := docsapp.RenderMarkdownFragmentWithBase(section.IntroductionMarkdown, repositoryURL+"/blob/"+releasecatalog.SourceCommit+"/")
			if err != nil {
				return nil, 0, fmt.Errorf("render %s %s introduction: %w", release.Version, section.Name, err)
			}
			entries := make([]map[string]any, 0, len(section.Entries))
			for _, entry := range section.Entries {
				content, err := docsapp.RenderMarkdownFragmentWithBase(entry.BodyMarkdown, repositoryURL+"/blob/"+releasecatalog.SourceCommit+"/")
				if err != nil {
					return nil, 0, fmt.Errorf(
						"render %s %s line %d: %w",
						release.Version,
						section.Name,
						entry.SourceLine,
						err,
					)
				}
				references := extractReferences(entry.Markdown)
				entries = append(entries, map[string]any{
					"content":     content,
					"references":  references,
					"hasRefs":     len(references) > 0,
					"sourceURL":   sourceLineURL(entry.SourceLine, release.SourcePath),
					"sourceLabel": "Source line " + strconv.Itoa(entry.SourceLine),
				})
				resultCount++
			}
			sections = append(sections, map[string]any{
				"name":            section.Name,
				"introduction":    introduction,
				"hasIntroduction": section.IntroductionMarkdown != "",
				"id":              versionAnchor(release) + "-" + slug(section.Name),
				"color":           categoryColor(section.Name),
				"impact":          sectionImpact(section.Name),
				"entries":         entries,
				"entryCount":      len(entries),
				"sourceURL":       sourceLineURL(section.SourceLine, release.SourcePath),
			})
		}

		narrativeTitle, narrativeBody := releaseNarrative(release)
		trail := historicalTrail(release)
		views = append(views, map[string]any{
			"id":              versionAnchor(release),
			"version":         displayVersion(release),
			"date":            releaseDate(release),
			"status":          string(release.Status),
			"statusLabel":     statusLabel(release),
			"open":            release.Status == releasecatalog.StatusUnreleased || fullIndex == 1,
			"sections":        sections,
			"entryCount":      releaseEntryCount(release),
			"impact":          releaseImpact(release),
			"impactClass":     releaseImpactClass(release),
			"narrativeTitle":  narrativeTitle,
			"narrativeBody":   narrativeBody,
			"hasNarrative":    narrativeBody != "",
			"evidenceURL":     releaseEvidenceURL(release),
			"codeURL":         releaseCodeURL(release, fullIndex),
			"sourceURL":       sourceLineURL(release.SourceLine, release.SourcePath),
			"previous":        adjacentVersion(fullIndex + 1),
			"next":            adjacentVersion(fullIndex - 1),
			"hasPrevious":     fullIndex+1 < len(catalog.Releases),
			"hasNext":         fullIndex > 0,
			"historicalTrail": trail,
			"hasTrail":        len(trail) > 0,
		})
	}
	return views, resultCount, nil
}

func renderFilterForm(query, category string, status releasecatalog.Status, categories []string) gosx.Node {
	categoryOptions := []gosx.Node{optionNode("All categories", "", category == "")}
	for _, name := range categories {
		categoryOptions = append(categoryOptions, optionNode(name, name, category == name))
	}
	statusOptions := []gosx.Node{
		optionNode("Released and unreleased", "", status == ""),
		optionNode("Released only", string(releasecatalog.StatusReleased), status == releasecatalog.StatusReleased),
		optionNode("Unreleased only", string(releasecatalog.StatusUnreleased), status == releasecatalog.StatusUnreleased),
	}
	categorySelectArgs := []any{
		gosx.Attrs(gosx.Attr("class", "change-select"), gosx.Attr("name", "category")),
	}
	for _, option := range categoryOptions {
		categorySelectArgs = append(categorySelectArgs, option)
	}
	statusSelectArgs := []any{
		gosx.Attrs(gosx.Attr("class", "change-select"), gosx.Attr("name", "status")),
	}
	for _, option := range statusOptions {
		statusSelectArgs = append(statusSelectArgs, option)
	}

	return server.Form(
		gosx.Attrs(
			gosx.Attr("class", "change-filters"),
			gosx.Attr("method", "get"),
			gosx.Attr("action", "/changelog"),
			gosx.Attr(server.NavigationFormModeAttr, "get"),
			gosx.Attr("role", "search"),
			gosx.Attr("aria-label", "Filter changelog"),
		),
		gosx.El("label", gosx.Attrs(gosx.Attr("class", "change-field change-query")),
			gosx.El("span", gosx.Attrs(gosx.Attr("class", "change-label")), gosx.Text("Search history")),
			gosx.El("input", gosx.Attrs(
				gosx.Attr("class", "change-input"),
				gosx.Attr("type", "search"),
				gosx.Attr("name", "q"),
				gosx.Attr("value", query),
				gosx.Attr("placeholder", "recovery, scanner, allocation…"),
				gosx.Attr("autocomplete", "off"),
			)),
		),
		gosx.El("label", gosx.Attrs(gosx.Attr("class", "change-field")),
			gosx.El("span", gosx.Attrs(gosx.Attr("class", "change-label")), gosx.Text("Category")),
			gosx.El("select", categorySelectArgs...),
		),
		gosx.El("label", gosx.Attrs(gosx.Attr("class", "change-field")),
			gosx.El("span", gosx.Attrs(gosx.Attr("class", "change-label")), gosx.Text("Release status")),
			gosx.El("select", statusSelectArgs...),
		),
		gosx.El("div", gosx.Attrs(gosx.Attr("class", "change-actions")),
			gosx.El("button",
				gosx.Attrs(gosx.Attr("class", "change-submit"), gosx.Attr("type", "submit")),
				gosx.Text("Explore"),
			),
			server.Link("/changelog",
				gosx.Attrs(gosx.Attr("class", "change-reset")),
				gosx.Text("Reset"),
			),
		),
	)
}

func optionNode(label, value string, selected bool) gosx.Node {
	attrs := []any{gosx.Attr("value", value)}
	if selected {
		attrs = append(attrs, gosx.BoolAttr("selected"))
	}
	return gosx.El("option", gosx.Attrs(attrs...), gosx.Text(label))
}

func catalogEntryCount(releases []releasecatalog.Release) int {
	total := 0
	for _, release := range releases {
		total += releaseEntryCount(release)
	}
	return total
}

func releaseEntryCount(release releasecatalog.Release) int {
	total := 0
	for _, section := range release.Sections {
		total += len(section.Entries)
	}
	return total
}

func catalogReleaseIndex(version string) int {
	for i, candidate := range catalog.Releases {
		if candidate.Version == version {
			return i
		}
	}
	return -1
}

func buildVersionLinks() []map[string]any {
	links := make([]map[string]any, 0, len(catalog.Releases))
	for _, release := range catalog.Releases {
		if releaseEntryCount(release) == 0 {
			continue
		}
		links = append(links, map[string]any{
			"href":    "#" + versionAnchor(release),
			"version": displayVersion(release),
			"status":  string(release.Status),
		})
	}
	return links
}

func versionAnchor(release releasecatalog.Release) string {
	return "release-" + slug(displayVersion(release))
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	dash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
			dash = false
		default:
			if !dash && out.Len() > 0 {
				out.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(out.String(), "-")
}

func displayVersion(release releasecatalog.Release) string {
	if release.Status == releasecatalog.StatusUnreleased {
		return "Unreleased"
	}
	return release.Tag
}

func releaseDate(release releasecatalog.Release) string {
	if release.Status == releasecatalog.StatusUnreleased {
		return "Current main"
	}
	return release.Date
}

func statusLabel(release releasecatalog.Release) string {
	if release.Status == releasecatalog.StatusUnreleased {
		return "Unreleased · may change"
	}
	return "Released · immutable"
}

func sourceLineURL(line int, sourcePath ...string) string {
	url := catalog.Source.URL
	if len(sourcePath) > 0 && sourcePath[0] != "" {
		url = repositoryURL + "/blob/" + releasecatalog.SourceCommit + "/" + sourcePath[0]
	}
	if line <= 0 {
		return url
	}
	return url + "#L" + strconv.Itoa(line)
}

func releaseEvidenceURL(release releasecatalog.Release) string {
	if release.Status == releasecatalog.StatusUnreleased {
		return catalog.Source.URL
	}
	return repositoryURL + "/releases/tag/" + release.Tag
}

func releaseCodeURL(release releasecatalog.Release, index int) string {
	if release.Status == releasecatalog.StatusUnreleased {
		return repositoryURL + "/compare/" + catalog.Source.LatestReleased + "...HEAD"
	}
	if index >= 0 && index+1 < len(catalog.Releases) {
		older := catalog.Releases[index+1]
		if older.Status == releasecatalog.StatusReleased {
			return repositoryURL + "/compare/" + older.Tag + "..." + release.Tag
		}
	}
	return repositoryURL + "/releases/tag/" + release.Tag
}

func adjacentVersion(index int) map[string]any {
	if index < 0 || index >= len(catalog.Releases) {
		return map[string]any{}
	}
	release := catalog.Releases[index]
	return map[string]any{
		"href":    "#" + versionAnchor(release),
		"version": displayVersion(release),
	}
}

func categoryColor(category string) string {
	switch strings.ToLower(category) {
	case "security":
		return "c-red"
	case "removed":
		return "c-orange"
	case "fixed":
		return "c-green"
	case "performance", "improved":
		return "c-cyan"
	case "added":
		return "c-violet"
	case "changed":
		return "c-yellow"
	case "known issues":
		return "c-pink"
	default:
		return "c-blue"
	}
}

func sectionImpact(category string) string {
	switch strings.ToLower(category) {
	case "security":
		return "Security update"
	case "removed":
		return "Review before upgrading"
	case "changed":
		return "Behavior can change"
	case "known issues":
		return "Known release limit"
	case "fixed":
		return "Defect correction"
	case "performance", "improved":
		return "Runtime efficiency"
	case "added":
		return "New capability"
	default:
		return "Project maintenance"
	}
}

func releaseImpact(release releasecatalog.Release) string {
	priority := map[string]int{
		"security": 7, "removed": 6, "changed": 5, "known issues": 4,
		"fixed": 3, "performance": 2, "improved": 2, "added": 1,
	}
	bestName := "Project maintenance"
	best := 0
	for _, section := range release.Sections {
		if score := priority[strings.ToLower(section.Name)]; score > best {
			best = score
			bestName = sectionImpact(section.Name)
		}
	}
	return bestName
}

func releaseImpactClass(release releasecatalog.Release) string {
	impact := releaseImpact(release)
	switch impact {
	case "Security update":
		return "impact-security"
	case "Review before upgrading":
		return "impact-removed"
	case "Behavior can change", "Known release limit":
		return "impact-review"
	case "Defect correction":
		return "impact-fixed"
	case "Runtime efficiency":
		return "impact-performance"
	case "New capability":
		return "impact-added"
	default:
		return "impact-maintenance"
	}
}

func releaseNarrative(release releasecatalog.Release) (string, string) {
	switch displayVersion(release) {
	case "v0.55.1":
		return "Restore Python speed and correct large C parses.",
			"This patch restores Python parsing speed on files without unpacking. It fixes false C errors beyond the string-call limit. It also reduces Scala alias work and extends the Dart reuse guard. Transient-error incremental trees remain unresolved. The linked notes state the measured results and their limits."
	case "v0.55.0":
		return "Correct Python trees and reduce pathological parse work.",
			"This release fixes Python escape spans and splat binding. It reduces pathological C# election work and bounds Make, HTTP, and Dart work. It also fixes query capture ordering and makes explicit route settings override the admission allowlist. The highlighter route option applies to injected parsers."
	case "v0.54.0":
		return "Production parsing by default, with reuse fixes.",
			"Version 0.54.0 makes compact parsing opt-in through GTS_ADMISSION_CANDIDATE=1. It adds FactProgram.ExtractInto, reduces scanner overhead, and fixes GLR cache invalidation, incremental token-source resume, reuse-budget stops, missing-edit fallback, and YAML error shapes. Groovy incremental calls use a fresh parse. Compact parser graduation remains incomplete."
	case "v0.53.0":
		return "Safer trees, restored speed, and closer C parity.",
			"This release fixes timeout, tree-handle, and incremental-reuse contract faults an audit found. It restores the same-width token-invariant shortcut behind authenticated proofs, so single-byte edits fall to about 127.5 microseconds. The default memory budget now scales with input size, and reserved-word and query-predicate handling match C in more cases. Compact parser graduation remains unfinished."
	case "v0.52.0":
		return "Safer edits with an explicit performance cost.",
			"This release disables the unsafe same-width shortcut while preserving ordinary subtree reuse and no-edit reuse. Recovery optimizations reduce full-parse allocations. The measured single-byte edit becomes about 1,964 times slower. Complete lexical dependency proofs remain required before restoring the shortcut. Compact parser graduation remains unfinished."
	case "v0.50.1":
		return "Single-language builds work again.",
			"Shared lexer helpers had moved into subset-gated files, so a build selecting one grammar could not compile. CI now sweeps every single-language subset build, which is how this class of break gets caught before a release rather than after."
	case "v0.50.0":
		return "The file outline grew to cover most of the language set.",
			"Tags-query coverage rose from 30 languages to 84, and a differential test now checks every resolved query against the official C runtime. TypeScript and TSX also stopped splitting a signed right-shift operator into two generic closers."
	case "v0.49.0":
		return "Compiled fact extraction arrived, and recovery got cheaper.",
			"FactProgram compiles definition, call, heritage, and import work into one pass. Recovery reduced allocations and time across the benchmark fleet, and Lean 4 joined the opt-in grammar set."
	case "v0.48.0":
		return "The parser campaign moved into an immutable release.",
			"The tag adds Swift corpus coverage, route evidence, recovery corrections, and bounded parser improvements."
	case "v0.47.1":
		return "An emergency correction restored valid Go recovery.",
			"Recovery reductions preserve deferred parent links during fresh parses. Complete Go trees survive final materialization. The invariant check still rejects invalid replacements."
	case "v0.47.0":
		return "The graduation campaign became measurable.",
			"This release sealed the full-parse receipts for the production and compact paths. It expanded the recovery evidence. The compact path stayed diagnostic."
	default:
		return "", ""
	}
}

func extractReferences(markdown string) []map[string]any {
	var references []map[string]any
	seen := make(map[string]struct{})
	appendReference := func(label, href, kind string) {
		if _, exists := seen[href]; exists {
			return
		}
		seen[href] = struct{}{}
		references = append(references, map[string]any{
			"label": label,
			"href":  href,
			"kind":  kind,
		})
	}

	for _, match := range pullRequestPattern.FindAllStringSubmatch(markdown, -1) {
		for _, number := range match[1:] {
			if number != "" {
				appendReference("PR #"+number, repositoryURL+"/pull/"+number, "pull request")
			}
		}
	}
	for _, match := range issuePattern.FindAllStringSubmatch(markdown, -1) {
		appendReference("Issue #"+match[1], repositoryURL+"/issues/"+match[1], "issue")
	}
	for _, match := range commitPattern.FindAllStringSubmatch(markdown, -1) {
		if _, external := nonRepositoryCommitIDs[match[1]]; external || isLongNumericID(match[1]) {
			continue
		}
		appendReference("Commit "+match[1], repositoryURL+"/commit/"+match[1], "commit")
	}
	for _, tag := range tagPattern.FindAllString(markdown, -1) {
		if !hasReleaseTag(tag) {
			continue
		}
		appendReference(tag, repositoryURL+"/releases/tag/"+tag, "release")
	}
	return references
}

func isLongNumericID(value string) bool {
	return len(value) >= 10 && strings.Trim(value, "0123456789") == ""
}

func hasReleaseTag(tag string) bool {
	for _, release := range catalog.Releases {
		if release.Tag == tag {
			return true
		}
	}
	return false
}

func historicalTrail(release releasecatalog.Release) []map[string]any {
	switch displayVersion(release) {
	case "v0.47.1":
		return []map[string]any{
			trailLink("Issue #490", "Go grammar regression", repositoryURL+"/issues/490"),
			trailLink("PR #495", "Recovery correction", repositoryURL+"/pull/495"),
			trailLink("PR #496", "Emergency release", repositoryURL+"/pull/496"),
		}
	default:
		return nil
	}
}

func campaignTrail() []map[string]any {
	return []map[string]any{
		trailLink("v0.48.0", "Swift corpus and route evidence", repositoryURL+"/releases/tag/v0.48.0"),
		trailLink("v0.47.1", "Emergency recovery fix", repositoryURL+"/releases/tag/v0.47.1"),
		trailLink("PR #505", "Retire skipped-error class", repositoryURL+"/pull/505"),
		trailLink("PR #508", "Retire recovery-action class", repositoryURL+"/pull/508"),
		trailLink("PR #510", "Retire root fallback class", repositoryURL+"/pull/510"),
		trailLink("PR #511", "Retire recovery materialization class", repositoryURL+"/pull/511"),
	}
}

func trailLink(label, description, href string) map[string]any {
	return map[string]any{
		"label":       label,
		"description": description,
		"href":        href,
	}
}
