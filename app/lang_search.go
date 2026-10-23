package docs

import (
	"strconv"
	"strings"

	"m31labs.dev/gosx"
)

// BuildLangSearch renders the language list with a server-side filter: a GET
// form that submits ?q=<text> to /docs/languages. It replaces the former
// GoSX island, so the page ships no script and needs no WebAssembly runtime.
// Names match on a case-insensitive substring, like the island did.
func BuildLangSearch(names []string, query string) gosx.Node {
	query = strings.TrimSpace(query)
	needle := strings.ToLower(query)
	tiles := make([]gosx.Node, 0, len(names))
	for i, name := range names {
		if needle != "" && !strings.Contains(strings.ToLower(name), needle) {
			continue
		}
		children := []gosx.Node{
			elWith("span", gosx.Attrs(gosx.Attr("class", "ldot "+dotPalette[i%len(dotPalette)]))),
			elWith("span", gosx.Attrs(gosx.Attr("class", "lname")), gosx.Text(name)),
		}
		if langTokenSourceSet[name] {
			children = append(children, elWith("span", gosx.Attrs(gosx.Attr("class", "lts")), gosx.Text("TS")))
		}
		tiles = append(tiles, elWith("div", gosx.Attrs(gosx.Attr("class", "langtile")), children...))
	}
	count := strconv.Itoa(len(tiles)) + " / " + strconv.Itoa(len(names))

	form := elWith("form", gosx.Attrs(
		gosx.Attr("class", "langbar"), gosx.Attr("method", "get"), gosx.Attr("action", "/docs/languages"),
		gosx.Attr("role", "search"), gosx.Attr("aria-label", "Filter languages"),
		gosx.Attr("data-gosx-form-mode", "get"),
	),
		elWith("input", gosx.Attrs(
			gosx.Attr("class", "search mono"), gosx.Attr("type", "search"), gosx.Attr("name", "q"),
			gosx.Attr("value", query), gosx.Attr("placeholder", "filter languages…  try 'script' or 'go'"),
			gosx.Attr("aria-label", "Filter languages"),
		)),
		elWith("button", gosx.Attrs(gosx.Attr("class", "change-submit"), gosx.Attr("type", "submit")), gosx.Text("Filter")),
		elWith("span", gosx.Attrs(gosx.Attr("class", "count mono"), gosx.Attr("role", "status")), gosx.Text(count)),
	)
	var grid gosx.Node
	if len(tiles) == 0 {
		grid = elWith("p", gosx.Attrs(gosx.Attr("class", "p mut")), gosx.Text("No language matches “"+query+"”."))
	} else {
		grid = elWith("div", gosx.Attrs(gosx.Attr("class", "langgrid")), tiles...)
	}
	return elWith("div", gosx.Attrs(gosx.Attr("class", "lang-island")), form, grid)
}
