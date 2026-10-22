package changelog

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/odvcencio/gotreesitter-docs/internal/releasecatalog"
	"m31labs.dev/gosx/route"
)

// The changelog is split by minor version: /changelog lists every release and
// /changelog/v0.55 renders the releases of that series in full, each keeping
// its release anchor (#release-v0-55-1). Rendering all 102 releases on one
// page made a 13,000-element document that no mobile budget could carry.

const unreleasedKey = "unreleased"

// minorKey maps a release to its page slug: "v0.55.1" -> "v0.55".
func minorKey(release releasecatalog.Release) string {
	if release.Status == releasecatalog.StatusUnreleased {
		return unreleasedKey
	}
	parts := strings.Split(strings.TrimPrefix(release.Tag, "v"), ".")
	if len(parts) < 2 {
		return strings.ToLower(release.Tag)
	}
	return "v" + parts[0] + "." + parts[1]
}

func minorLabel(key string) string {
	if key == unreleasedKey {
		return "Unreleased"
	}
	return key
}

func minorHref(key string) string { return "/changelog/" + key }

// minorKeys lists the page slugs in catalog order (newest first), skipping
// releases with no entries, as the old version index did.
func minorKeys() []string {
	var keys []string
	seen := map[string]bool{}
	for _, release := range catalog.Releases {
		if releaseEntryCount(release) == 0 {
			continue
		}
		key := minorKey(release)
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

// releaseAnchors returns the two ids a release row carries on the index page:
// the card id used since the single-page layout and the dotted tag.
func releaseAnchors(release releasecatalog.Release) (string, string) {
	return versionAnchor(release), displayVersion(release)
}

// releaseSummary is the one-line index summary: the release narrative title
// when the page carries one, otherwise a count of its sections.
func releaseSummary(release releasecatalog.Release) string {
	if title, _ := releaseNarrative(release); title != "" {
		return title
	}
	parts := make([]string, 0, len(release.Sections))
	for _, section := range release.Sections {
		if n := len(section.Entries); n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", section.Name, n))
		}
	}
	if len(parts) == 0 {
		return "No entries"
	}
	return strings.Join(parts, " · ")
}

// queryString re-encodes the active filters so the release page opened from a
// filtered index shows the same matches.
func queryString(query, category string, status releasecatalog.Status) string {
	values := url.Values{}
	if query != "" {
		values.Set("q", query)
	}
	if category != "" {
		values.Set("category", category)
	}
	if status != "" {
		values.Set("status", string(status))
	}
	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}

// buildIndexGroups groups the filtered releases by minor version for /changelog.
func buildIndexGroups(filtered []releasecatalog.Release, qs string) []map[string]any {
	var groups []map[string]any
	var current map[string]any
	var rows []map[string]any
	flush := func() {
		if current != nil {
			current["rows"] = rows
			current["releaseCount"] = len(rows)
			groups = append(groups, current)
		}
	}
	lastKey := ""
	for _, release := range filtered {
		count := releaseEntryCount(release)
		if count == 0 {
			continue
		}
		key := minorKey(release)
		if key != lastKey {
			flush()
			lastKey = key
			rows = nil
			current = map[string]any{
				"id":    "series-" + slug(key),
				"label": minorLabel(key),
				"href":  minorHref(key) + qs,
			}
		}
		id, alt := releaseAnchors(release)
		rows = append(rows, map[string]any{
			"id":          id,
			"altID":       alt,
			"version":     displayVersion(release),
			"date":        releaseDate(release),
			"summary":     releaseSummary(release),
			"impact":      releaseImpact(release),
			"impactClass": releaseImpactClass(release),
			"entryCount":  count,
			"href":        minorHref(key) + qs + "#" + versionAnchor(release),
		})
	}
	flush()
	return groups
}

// LoadRelease backs /changelog/{series}: the releases of one minor version,
// rendered in full and open.
func LoadRelease(ctx *route.RouteContext, _ route.FilePage) (any, error) {
	if catalogErr != nil {
		return nil, fmt.Errorf("load changelog catalog: %w", catalogErr)
	}
	key := strings.ToLower(strings.Trim(ctx.Param("series"), "/"))
	keys := minorKeys()
	position := -1
	for i, candidate := range keys {
		if strings.ToLower(candidate) == key {
			position = i
			key = candidate
		}
	}
	if position < 0 {
		return nil, route.NotFound("no changelog page for " + ctx.Param("series"))
	}
	query := strings.TrimSpace(ctx.Query("q"))
	category := strings.TrimSpace(ctx.Query("category"))
	status := normalizeStatus(ctx.Query("status"))
	filtered := catalog.Filter(releasecatalog.Filter{Query: query, Category: category, Status: status})
	var inSeries []releasecatalog.Release
	for _, release := range filtered {
		if minorKey(release) == key && releaseEntryCount(release) > 0 {
			inSeries = append(inSeries, release)
		}
	}
	views, resultCount, err := buildReleaseViews(inSeries)
	if err != nil {
		return nil, err
	}
	for _, view := range views {
		view["open"] = true
	}
	hasFilters := query != "" || category != "" || status != ""
	data := map[string]any{
		"series":      key,
		"label":       minorLabel(key),
		"releases":    views,
		"hasResults":  len(views) > 0,
		"resultCount": resultCount,
		"hasFilters":  hasFilters,
		"indexHref":   "/changelog" + queryString(query, category, status),
		"sourceURL":   catalog.Source.URL,
		"hasOlder":    position+1 < len(keys),
		"hasNewer":    position > 0,
	}
	if position+1 < len(keys) {
		data["olderLabel"] = minorLabel(keys[position+1])
		data["olderHref"] = minorHref(keys[position+1])
	}
	if position > 0 {
		data["newerLabel"] = minorLabel(keys[position-1])
		data["newerHref"] = minorHref(keys[position-1])
	}
	return data, nil
}
