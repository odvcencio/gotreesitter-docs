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
)

func init() {
	docsapp.RegisterStaticDocsPage(
		"Changelog",
		"Explore every GoTreeSitter release, current unreleased work, upgrade impact, and source evidence.",
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
		title       = "Changelog — GoTreeSitter"
		description = "Explore GoTreeSitter releases, current work, upgrade impact, and source evidence."
		siteURL     = "https://gotreesitter.m31labs.dev"
	)
	image := server.MediaAsset{
		URL:    siteURL + docsapp.PublicAssetURL("social/changelog.png"),
		Width:  1200,
		Height: 630,
		Alt:    "GoTreeSitter Changelog with an abstract syntax-tree release timeline.",
		Type:   "image/png",
	}
	return server.Metadata{
		Title:        server.Title{Absolute: title},
		Description:  description,
		MetadataBase: siteURL,
		Alternates:   &server.Alternates{Canonical: siteURL + "/changelog"},
		OpenGraph: &server.OpenGraph{
			Type:        "website",
			URL:         siteURL + "/changelog",
			SiteName:    "GoTreeSitter",
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
			entries := make([]map[string]any, 0, len(section.Entries))
			for _, entry := range section.Entries {
				content, err := docsapp.RenderMarkdownFragment(entry.BodyMarkdown)
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
					"sourceURL":   sourceLineURL(entry.SourceLine),
					"sourceLabel": "Source line " + strconv.Itoa(entry.SourceLine),
				})
				resultCount++
			}
			sections = append(sections, map[string]any{
				"name":       section.Name,
				"id":         versionAnchor(release) + "-" + slug(section.Name),
				"color":      categoryColor(section.Name),
				"impact":     sectionImpact(section.Name),
				"entries":    entries,
				"entryCount": len(entries),
				"sourceURL":  sourceLineURL(section.SourceLine),
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
			"sourceURL":       sourceLineURL(release.SourceLine),
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

func sourceLineURL(line int) string {
	if line <= 0 {
		return catalog.Source.URL
	}
	return catalog.Source.URL + "#L" + strconv.Itoa(line)
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
	case "Unreleased":
		return "The compatibility layer is shrinking by parser-owned classes.",
			"Current main retires four compatibility classes across ten grammars. Compact and forest routes keep their fail-closed checks."
	case "v0.47.1":
		return "An emergency correction restored valid Go recovery.",
			"Recovery reductions preserve deferred parent links during fresh parses. Complete Go trees survive final materialization, while the invariant guard still rejects invalid replacements."
	case "v0.47.0":
		return "The graduation campaign became measurable.",
			"This release sealed production and compact full-parse receipts, expanded recovery evidence, and kept the compact path diagnostic rather than presenting it as the public parser."
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
		if strings.IndexAny(match[1], "abcdef") >= 0 {
			appendReference("Commit "+match[1], repositoryURL+"/commit/"+match[1], "commit")
		}
	}
	for _, tag := range tagPattern.FindAllString(markdown, -1) {
		appendReference(tag, repositoryURL+"/releases/tag/"+tag, "release")
	}
	return references
}

func historicalTrail(release releasecatalog.Release) []map[string]any {
	switch displayVersion(release) {
	case "Unreleased":
		return []map[string]any{
			trailLink("PR #505", "Robot + Scheme class", repositoryURL+"/pull/505"),
			trailLink("PR #506", "Compact allocation", repositoryURL+"/pull/506"),
			trailLink("PR #507", "Named recovery error", repositoryURL+"/pull/507"),
			trailLink("PR #508", "Forth + Luau class", repositoryURL+"/pull/508"),
			trailLink("PR #509", "Stop-check allocation", repositoryURL+"/pull/509"),
			trailLink("PR #510", "Hurl + INI class", repositoryURL+"/pull/510"),
			trailLink("PR #511", "Four-grammar recovery class", repositoryURL+"/pull/511"),
		}
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
