package docs

import (
	"sort"
	"strconv"
	"strings"
)

// docsNavGroupOrder fixes the sidebar section order. A doc whose frontmatter
// nav_group doesn't match one of these lands in a trailing "More" section
// instead of silently disappearing from the nav.
var docsNavGroupOrder = []string{
	"Introduction",
	"Using the Parser",
	"Languages",
	"Internals",
	"Project",
}

type docsNavLink struct {
	Slug  string
	Label string
	Order int
}

// buildDocsNavGroups reads the loaded docs collection's frontmatter and
// returns the sidebar nav data: groups in docsNavGroupOrder, each sorted by
// the `order` frontmatter field. Nothing here is hand-maintained — dropping
// a new content/docs/*.md file with a valid nav_group is enough for it to
// show up.
func buildDocsNavGroups() []map[string]any {
	docs := docsLibrary.Collection("docs")

	byGroup := make(map[string][]docsNavLink, len(docsNavGroupOrder)+1)
	for _, doc := range docs {
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
			})
		}
		groups = append(groups, map[string]any{
			"name":  name,
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
