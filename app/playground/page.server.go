package playground

import (
	"embed"
	"net/url"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	"github.com/odvcencio/gotreesitter-docs/internal/playgroundsamples"
	"github.com/odvcencio/gotreesitter-docs/internal/playgroundshare"
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
	state := resolvePlaygroundState(ctx.Query("lang"), ctx.Query("sample"), ctx.Request.URL.Query())
	return map[string]any{
		"language":        state.Language,
		"sample":          state.SampleName,
		"source":          state.Source,
		"query":           state.Query,
		"shared":          state.Shared,
		"shareError":      state.ShareError,
		"gtsVersion":      docsapp.PlaygroundGTSVersion(),
		"wasmURL":         docsapp.PublicAssetURL("playground/runtime.wasm"),
		"grammarIndexURL": docsapp.PublicAssetURL("playground/grammars/index.json"),
	}, nil
}

// shareErrorMessage is shown when a share link cannot be opened. The page
// never answers 400 for a broken link; it opens the sample instead.
const shareErrorMessage = "This share link could not be opened; it was too large or damaged. The sample is shown instead."

type playgroundState struct {
	Language   string
	SampleName string
	Source     string
	Query      string
	Shared     bool
	ShareError string
}

// resolvePlaygroundState picks what the page opens with: a valid share link
// wins over ?sample=, a broken one falls back to the sample with a notice.
func resolvePlaygroundState(lang, sampleName string, values url.Values) playgroundState {
	language, name, sample := initialPlaygroundSample(lang, sampleName)
	state := playgroundState{Language: language, SampleName: name, Source: sample.Source, Query: sample.Query}
	share, shared, err := playgroundshare.Decode(values)
	switch {
	case err != nil:
		state.ShareError = shareErrorMessage
	case shared:
		state.Shared = true
		state.Source = share.Source
		if share.Query != "" {
			state.Query = share.Query
		}
	}
	return state
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
