package docs

import (
	"strings"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	"m31labs.dev/gosx/route"
	"m31labs.dev/gosx/server"
)

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

				node, err := doc.Render(nil)
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
