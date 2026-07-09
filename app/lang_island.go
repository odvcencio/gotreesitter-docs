package docs

// Island 2 — the 206-language search filter (design/PHASE-B-NOTES.md).
//
// A single hand-built island Program owns the whole `.langbar` +
// `.langgrid`: a `query` signal, a `<input oninput>` handler that sets it,
// and a `NodeForEach` over the `langs` prop (206 entries sourced from
// content/docs/languages.md's ```langlist block — same single source of
// truth renderLangGrid used before this pass) that toggles each tile's
// class between "langtile" and "langtile hidden" via a per-tile
// Contains(ToLower(name), ToLower(query)) expression, plus a live count via
// Len(Filter(langs, ...)). The Program itself carries no language data —
// only the JSON props do — so it's static/reusable across requests.
//
// This produces exactly the markup renderLangGrid rendered statically
// before (.langtile/.ldot/.lname/.lts unchanged), just wrapped as a real
// GoSX island (data-gosx-island + a compiled program served from
// /gosx/islands/LangSearch.json) instead of inert HTML.

import (
	"sync"

	"m31labs.dev/gosx"
	islandprogram "m31labs.dev/gosx/island/program"
	"m31labs.dev/gosx/server"
)

const langSearchIslandName = "LangSearch"

// LangSearchProgramPath is where the compiled program is served from —
// mounted in main.go, referenced here via SetProgramAsset.
const LangSearchProgramPath = "/gosx/islands/LangSearch.json"

var (
	langSearchProgramOnce sync.Once
	langSearchProgramVal  *islandprogram.Program
)

// LangSearchProgram returns the (process-wide constant) compiled island
// program for the language search filter. Exported so main.go can serve it
// at langSearchProgramPath without rebuilding it per request.
func LangSearchProgram() *islandprogram.Program {
	langSearchProgramOnce.Do(func() {
		langSearchProgramVal = buildLangSearchProgram()
	})
	return langSearchProgramVal
}

func buildLangSearchProgram() *islandprogram.Program {
	b := newProgBuilder()

	b.signal("query", islandprogram.TypeString, b.lit(""))
	b.handler("setQuery", b.signalSet("query", b.eventGet("value"), islandprogram.TypeString))

	// ---- .langbar: search input + live count -----------------------
	searchInput := b.el("input", []islandprogram.Attr{
		attrStatic("class", "search mono"),
		attrStatic("type", "text"),
		attrStatic("placeholder", "filter 206 languages…  try 'script' or 'go'"),
		attrStatic("aria-label", "Filter languages"),
		attrEvent("input", "setQuery"),
	})

	item := b.propGet("_item")
	matchPred := b.contains(b.toLower(b.field(item, "name")), b.toLower(b.signalGet("query", islandprogram.TypeString)))
	visibleCount := b.length(b.filter(b.propGet("langs"), matchPred))

	countSpan := b.el("span", []islandprogram.Attr{attrStatic("class", "count mono")},
		b.exprNode(visibleCount),
		b.text(" / 206"),
	)

	langbar := b.el("div", []islandprogram.Attr{attrStatic("class", "langbar")}, searchInput, countSpan)

	// ---- .langgrid: one tile per language, class toggled by match --
	tileItem := b.propGet("l")
	tileMatch := b.contains(b.toLower(b.field(tileItem, "name")), b.toLower(b.signalGet("query", islandprogram.TypeString)))
	tileClass := b.cond(tileMatch, b.lit("langtile"), b.lit("langtile hidden"))

	dot := b.el("span", []islandprogram.Attr{attrExpr("class", b.concat(b.lit("ldot "), b.field(tileItem, "color")))})
	name := b.el("span", []islandprogram.Attr{attrStatic("class", "lname")}, b.exprNode(b.field(tileItem, "name")))
	tsBadge := b.condNode(b.field(tileItem, "ts"),
		b.el("span", []islandprogram.Attr{attrStatic("class", "lts")}, b.text("TS")),
	)

	tile := b.el("div", []islandprogram.Attr{attrExpr("class", tileClass)}, dot, name, tsBadge)
	grid := b.el("div", []islandprogram.Attr{attrStatic("class", "langgrid")},
		b.forEachNode(b.propGet("langs"), "l", tile),
	)

	root := b.el("div", []islandprogram.Attr{attrStatic("class", "lang-island")}, langbar, grid)

	return b.build(langSearchIslandName, root)
}

// langDotPalette/langTokenSource mirror render_blocks.go's dotPalette /
// langTokenSourceSet exactly (same 8-color rotation, same 5-language
// hand-written-token-source set) so the island's props produce byte-for-byte
// the same tile decoration the old static renderLangGrid did.
func langSearchProps(names []string) map[string]any {
	langs := make([]map[string]any, 0, len(names))
	for i, name := range names {
		langs = append(langs, map[string]any{
			"name":  name,
			"color": dotPalette[i%len(dotPalette)],
			"ts":    langTokenSourceSet[name],
		})
	}
	return map[string]any{"langs": langs}
}

// BuildLangGridIsland wires the language-search island into a page: it
// registers the compiled program's asset location on the page's runtime
// (server.PageRuntime — the same registry route.RouteContext.Runtime()
// returns) and returns the server-rendered, hydration-ready island node.
// names is the raw 206-language list (order preserved) as already sourced
// from content/docs/languages.md's ```langlist block.
func BuildLangGridIsland(rt *server.PageRuntime, names []string) gosx.Node {
	if rt == nil {
		return gosx.Text("")
	}
	rt.SetProgramAsset(langSearchIslandName, LangSearchProgramPath, "json", "")
	return rt.Island(LangSearchProgram(), langSearchProps(names))
}
