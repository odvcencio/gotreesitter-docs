package docs

import (
	"fmt"

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
				version := PlaygroundGTSVersion()
				facts := PinnedRegistryFacts()
				return route.FileTemplateBindings{
					Values: map[string]any{
						"features":       landingFeatures(version, facts),
						"gtsVersion":     version,
						"gtsReleaseDate": PlaygroundGTSReleaseDate(),
						"registry":       facts,
					},
					Funcs: map[string]any{
						"parserDemo": func() gosx.Node { return landingParserDemoNode(ctx.Nonce()) },
					},
				}
			},
		},
	)
}

// landingFeatures backs the compact feature grid on the landing page. The
// registry card derives its numbers from the build, not from prose.
func landingFeatures(version string, facts map[string]any) []map[string]any {
	registry := fmt.Sprintf("The pinned %s registry includes %d languages, with %d Go external scanners and %d token sources.",
		version, facts["grammars"], facts["scanners"], facts["tokenSources"])
	return []map[string]any{
		{"tok": "Go", "ttl": "No CGo in the runtime", "body": "The parser and query engine run as Go code. The project is released under the MIT license.", "color": "c-cyan"},
		{"tok": "206", "ttl": "206 grammar entries", "body": registry, "color": "c-violet"},
		{"tok": ".scm", "ttl": "Tree-sitter queries", "body": "Use familiar query patterns to find named nodes and captures in a parsed tree.", "color": "c-blue"},
		{"tok": "Δ", "ttl": "Incremental parsing", "body": "Apply edits to an existing tree and reparse the changed source.", "color": "c-green"},
	}
}
