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
	language, sampleName, sample := initialPlaygroundSample(ctx.Query("lang"), ctx.Query("sample"))
	return map[string]any{
		"language":        language,
		"sample":          sampleName,
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

// initialPlaygroundSample resolves ?lang= and ?sample=. It returns the language,
// the sample name that applied ("add" or "greet") and the sample itself.
// Unknown languages fall back to Go; unknown sample names and languages
// without a named sample fall back to the greet starter.
func initialPlaygroundSample(requestedLanguage, requestedSample string) (string, string, playgroundsamples.Sample) {
	language := requestedLanguage
	if _, ok := playgroundLanguageNames[language]; !ok {
		language = "go"
	}
	if requestedSample == playgroundsamples.SampleAdd {
		if sample, ok := playgroundsamples.Named(requestedSample, language); ok {
			return language, playgroundsamples.SampleAdd, sample
		}
	}
	sample, ok := playgroundsamples.ForIndexLanguage(language)
	if !ok {
		sample = playgroundsamples.Sample{}
	}
	return language, playgroundsamples.SampleGreet, sample
}
