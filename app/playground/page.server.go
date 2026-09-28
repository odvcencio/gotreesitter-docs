package playground

import (
	"embed"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	"github.com/odvcencio/gotreesitter-docs/internal/playgroundsamples"
	"github.com/odvcencio/gotreesitter/grammars"
	"m31labs.dev/gosx/route"
	"m31labs.dev/gosx/server"
)

//go:embed page.gsx
var pageSource embed.FS

func init() {
	docsapp.RegisterStaticDocsPage(
		"Playground",
		"Parse source and run tree-sitter queries locally in a GoSX-managed WebAssembly engine.",
		"/playground",
		route.FileModuleOptions{
			Load: loadPlayground,
			Metadata: func(ctx *route.RouteContext, page route.FilePage, data any) (server.Metadata, error) {
				return server.Metadata{
					Links: []server.LinkTag{{
						Rel:  "stylesheet",
						Href: docsapp.PublicAssetURL("playground/playground.css"),
					}},
				}, nil
			},
		},
	)
}

func loadPlayground(ctx *route.RouteContext, _ route.FilePage) (any, error) {
	language, sample := initialPlaygroundSample(ctx.Query("lang"))
	return map[string]any{
		"language":        language,
		"source":          sample.Source,
		"query":           sample.Query,
		"gtsVersion":      docsapp.PlaygroundGTSVersion(),
		"wasmURL":         docsapp.PublicAssetURL("playground/runtime.wasm"),
		"grammarIndexURL": docsapp.PublicAssetURL("playground/grammars/index.json"),
	}, nil
}

var playgroundLanguageNames = func() map[string]struct{} {
	entries := grammars.AllLanguages()
	allowed := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		allowed[entry.Name] = struct{}{}
	}
	return allowed
}()

func initialPlaygroundSample(requested string) (string, playgroundsamples.Sample) {
	if _, ok := playgroundLanguageNames[requested]; !ok {
		requested = "go"
	}
	sample, ok := playgroundsamples.ForIndexLanguage(requested)
	if !ok {
		sample = playgroundsamples.Sample{}
	}
	return requested, sample
}
