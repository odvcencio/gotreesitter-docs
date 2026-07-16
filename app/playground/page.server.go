package playground

import (
	"embed"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	"m31labs.dev/gosx/route"
	"m31labs.dev/gosx/server"
)

//go:embed page.gsx
var pageSource embed.FS

const initialGoSource = `package main

import "fmt"

func greet(name string) string {
	return "hey, " + name
}

func main() {
	fmt.Println(greet("gopher"))
}
`

const initialGoQuery = `(function_declaration name: (identifier) @function)`

func init() {
	docsapp.RegisterStaticDocsPage(
		"Playground",
		"Parse source and run tree-sitter queries locally in a GoSX-managed WebAssembly engine.",
		route.FileModuleOptions{
			Load: loadPlayground,
			Metadata: func(ctx *route.RouteContext, page route.FilePage, data any) (server.Metadata, error) {
				return server.Metadata{
					Title: server.Title{Absolute: "Playground — gotreesitter"},
					Links: []server.LinkTag{{
						Rel:  "stylesheet",
						Href: docsapp.PublicAssetURL("playground/playground.css"),
					}},
				}, nil
			},
		},
	)
}

func loadPlayground(_ *route.RouteContext, _ route.FilePage) (any, error) {
	return map[string]any{
		"source":          initialGoSource,
		"query":           initialGoQuery,
		"gtsVersion":      docsapp.PlaygroundGTSVersion(),
		"wasmURL":         docsapp.PublicAssetURL("playground/runtime.wasm"),
		"grammarIndexURL": docsapp.PublicAssetURL("playground/grammars/index.json"),
	}, nil
}
