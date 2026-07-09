package docs

import (
	"sort"
	"strconv"
	"strings"
)

// docsNavGroupOrder fixes the sidebar section order and is the grouping key
// each content/docs/*.md file's frontmatter `nav_group` must match. A doc
// whose frontmatter nav_group doesn't match one of these lands in a
// trailing "More" section instead of silently disappearing from the nav.
//
// These keys are left exactly as the content frontmatter already has them
// (Phase A does not touch content/docs/*.md). docsNavGroupLabels below maps
// each key to the label design/GoTreeSitter-Docs.html actually shows in its
// sidebar (`.navsec` text): "Start here", "Guides", "Grammar authoring",
// "Reference", "Project" — a display-only rename.
var docsNavGroupOrder = []string{
	"Introduction",
	"Using the Parser",
	"Languages",
	"Internals",
	"Project",
}

var docsNavGroupLabels = map[string]string{
	"Introduction":     "Start here",
	"Using the Parser": "Guides",
	"Languages":        "Grammar authoring",
	"Internals":        "Reference",
	"Project":          "Project",
}

// docsNavDotPalette is design.css's 8-color `.ndot` palette
// (.c-violet/.c-blue/.c-cyan/.c-green/.c-yellow/.c-orange/.c-red/.c-pink).
// It's the deterministic fallback for any doc slug missing from
// docsNavDotColors, so a new content/docs/*.md page always gets a dot
// color instead of an empty class.
var docsNavDotPalette = []string{
	"c-violet", "c-blue", "c-cyan", "c-green", "c-yellow", "c-orange", "c-red", "c-pink",
}

// docsNavDotColors assigns each of the 14 shipped docs pages a design
// palette color for its sidebar `.ndot`. Where a page has a direct analog
// in design/GoTreeSitter-Docs.html's own navDefs (e.g. queries,
// incremental-parsing, external-scanners), the color matches the design;
// the rest are chosen to stay visually distinct within their own nav
// section.
var docsNavDotColors = map[string]string{
	"introduction":             "c-pink",
	"getting-started":          "c-yellow",
	"syntax-trees-and-nodes":   "c-blue",
	"tree-cursors":             "c-cyan",
	"queries":                  "c-violet",
	"incremental-parsing":      "c-green",
	"languages":                "c-cyan",
	"authoring-languages":      "c-violet",
	"external-scanners":        "c-red",
	"recovery-and-correctness": "c-violet",
	"performance":              "c-blue",
	"architecture":             "c-orange",
	"playground":               "c-orange",
	"contributing":             "c-green",
}

type docsNavLink struct {
	Slug  string
	Label string
	Order int
	Color string
}

// buildDocsNavGroups reads the loaded docs collection's frontmatter and
// returns the sidebar nav data: groups in docsNavGroupOrder (relabeled via
// docsNavGroupLabels), each sorted by the `order` frontmatter field, each
// item carrying its `.ndot` color. Nothing here is hand-maintained beyond
// the color/label tables above — dropping a new content/docs/*.md file with
// a valid nav_group is enough for it to show up.
func buildDocsNavGroups() []map[string]any {
	docs := docsLibrary.Collection("docs")

	byGroup := make(map[string][]docsNavLink, len(docsNavGroupOrder)+1)
	for i, doc := range docs {
		label := strings.TrimSpace(doc.Frontmatter["title"])
		if label == "" {
			label = doc.Slug
		}
		order, _ := strconv.Atoi(strings.TrimSpace(doc.Frontmatter["order"]))

		group := strings.TrimSpace(doc.Frontmatter["nav_group"])
		if !docsNavGroupKnown(group) {
			group = "More"
		}
		byGroup[group] = append(byGroup[group], docsNavLink{
			Slug:  doc.Slug,
			Label: label,
			Order: order,
			Color: docsNavDotColor(doc.Slug, i),
		})
	}

	names := append([]string(nil), docsNavGroupOrder...)
	if _, ok := byGroup["More"]; ok {
		names = append(names, "More")
	}

	groups := make([]map[string]any, 0, len(names))
	for _, name := range names {
		links := byGroup[name]
		if len(links) == 0 {
			continue
		}
		sort.SliceStable(links, func(i, j int) bool {
			if links[i].Order != links[j].Order {
				return links[i].Order < links[j].Order
			}
			return links[i].Label < links[j].Label
		})
		items := make([]map[string]any, 0, len(links))
		for _, l := range links {
			items = append(items, map[string]any{
				"slug":  l.Slug,
				"label": l.Label,
				"color": l.Color,
			})
		}
		groups = append(groups, map[string]any{
			"name":  docsNavGroupLabel(name),
			"items": items,
		})
	}
	return groups
}

func docsNavGroupKnown(name string) bool {
	for _, candidate := range docsNavGroupOrder {
		if candidate == name {
			return true
		}
	}
	return false
}

func docsNavGroupLabel(name string) string {
	if label, ok := docsNavGroupLabels[name]; ok {
		return label
	}
	return name
}

func docsNavDotColor(slug string, fallbackIndex int) string {
	if color, ok := docsNavDotColors[slug]; ok {
		return color
	}
	return docsNavDotPalette[fallbackIndex%len(docsNavDotPalette)]
}
