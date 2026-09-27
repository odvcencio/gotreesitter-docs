package docs

import (
	"m31labs.dev/gosx"
	"m31labs.dev/gosx/route"
)

func init() {
	RegisterStaticDocsPage(
		"Overview",
		"A pure-Go tree-sitter runtime with 206 built-in grammars, incremental parsing, and no CGo dependency.",
		"/",
		route.FileModuleOptions{
			Bindings: func(ctx *route.RouteContext, page route.FilePage, data any) route.FileTemplateBindings {
				return route.FileTemplateBindings{
					Values: map[string]any{
						"features": landingFeatures,
					},
					Funcs: map[string]any{
						"parserDemo": func() gosx.Node { return landingParserDemoNode(ctx.Nonce()) },
					},
				}
			},
		},
	)
}

// landingFeatures backs the compact feature grid on the landing page.
var landingFeatures = []map[string]any{
	{"tok": "Go", "ttl": "No CGo in the runtime", "body": "The parser and query engine run as Go code. The project is released under the MIT license.", "color": "c-cyan"},
	{"tok": "206", "ttl": "206 grammar entries", "body": "The pinned v0.55.1 registry includes 206 languages, with 119 Go external scanners and 7 token sources.", "color": "c-violet"},
	{"tok": ".scm", "ttl": "Tree-sitter queries", "body": "Use familiar query patterns to find named nodes and captures in a parsed tree.", "color": "c-blue"},
	{"tok": "Δ", "ttl": "Incremental parsing", "body": "Apply edits to an existing tree and reparse the changed source.", "color": "c-green"},
}
