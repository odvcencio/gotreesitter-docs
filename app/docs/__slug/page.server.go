package docs

import (
	"strings"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	"m31labs.dev/gosx"
	"m31labs.dev/gosx/content"
	"m31labs.dev/gosx/route"
	"m31labs.dev/gosx/server"
)

// docsRenderer picks the renderer for one docs/{slug} page. Two slugs carry
// Phase B2's real client islands (design/PHASE-B-NOTES.md) instead of plain
// markdown-to-HTML:
//   - "languages" — Island 2, the 206-language search filter, spliced in
//     place of the ```langlist block that used to render a static grid.
//   - "playground" — Island 1, the playground, replacing the "coming soon"
//     prose entirely.
//
// Every other slug renders exactly as before (docsapp.RenderDesignDoc,
// unchanged). ctx.Runtime() is the per-request server.PageRuntime shared
// with router.renderPage's automatic head injection (route/route.go), so
// registering an island here is enough for its bootstrap/manifest scripts
// to show up in the response with no other wiring.
func docsRenderer(slug string, ctx *route.RouteContext) content.RendererFunc {
	switch slug {
	case "languages":
		return docsapp.RenderDesignDocWithLangIsland(func(names []string) gosx.Node {
			return docsapp.BuildLangGridIsland(ctx.Runtime(), names)
		})
	case "playground":
		return docsapp.RenderDesignDocIntroPlus(docsapp.BuildPlaygroundIsland(ctx.Runtime()))
	default:
		return docsapp.RenderDesignDoc
	}
}

// This directory uses the double-underscore catch-all convention
// (__slug -> {slug...}) instead of the legacy [...slug] bracket syntax. The
// bracket form still routes correctly, but a real .go file cannot live
// inside a [...] directory: `go build`/`go list` reject "[" as an invalid
// import-path character (verified directly), and gosx's own modules.go
// generator (cmd/gosx/modules_sync.go) skips [-prefixed directories for the
// same reason. __slug is Go-import-path safe, so page.server.go can live
// beside page.gsx here and still gets picked up by go build and the
// modules.go sync step.
func init() {
	docsapp.RegisterDocsPage(
		"Docs",
		"GoTreeSitter documentation.",
		route.FileModuleOptions{
			Load: func(ctx *route.RouteContext, page route.FilePage) (any, error) {
				slug := strings.Trim(ctx.Param("slug"), "/")
				if slug == "" {
					return nil, route.NotFound("missing docs slug")
				}

				doc, ok := docsapp.DocsLibrary().BySlug("docs", slug)
				if !ok {
					return nil, route.NotFound("no docs page for " + slug)
				}

				node, err := doc.Render(docsRenderer(slug, ctx))
				if err != nil {
					return nil, err
				}

				title := strings.TrimSpace(doc.Frontmatter["title"])
				if title == "" {
					title = slug
				}

				return map[string]any{
					"title":       title,
					"description": doc.Frontmatter["description"],
					"content":     node,
				}, nil
			},
			Metadata: func(ctx *route.RouteContext, page route.FilePage, data any) (server.Metadata, error) {
				values, _ := data.(map[string]any)
				title, _ := values["title"].(string)
				description, _ := values["description"].(string)
				return server.Metadata{
					Title:       server.Title{Default: title},
					Description: description,
				}, nil
			},
		},
	)
}
