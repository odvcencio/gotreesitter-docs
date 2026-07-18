package authoring

import (
	"embed"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	"github.com/odvcencio/gotreesitter/grammargen"
	"m31labs.dev/gosx/route"
	"m31labs.dev/gosx/server"
)

//go:embed page.gsx
var pageSource embed.FS

// defaultBaseName matches cmd/authoring-wasm's defaultBaseName: the base
// picker auto-selects it once public/authoring/bases/index.json loads, and
// initialDeltaJSON below is written as a delta *against this specific base*
// — calc is grammargen's fastest base (sub-millisecond native LR generation
// — see cmd/build-authoring-wasm's baseGrammars doc), so the very first
// compile on page load is effectively instant, exactly like Phase 0/1's
// seed grammar was.
const defaultBaseName = "calc"

// initialDeltaJSON is Phase 2's seed delta: it inherits the calc base
// unchanged except for two things, built directly with grammargen's Go DSL
// (rather than hand-transcribed grammar.json) so it can never drift out of
// sync with what calc's "expression" rule actually looks like:
//
//   - "expression" is overridden to add one more alternative: a SYMBOL
//     reference to a brand-new "power" rule (calc has no exponent operator).
//     This is exactly the shape a real author writes when wiring a new rule
//     into an existing choice point — you cannot append to a CHOICE without
//     restating it, so the delta necessarily "overrides" expression even
//     though every original alternative is preserved byte-for-byte.
//   - "power" is a brand-new rule: `expression "**" expression`,
//     right-associative, with higher precedence than every existing
//     operator (so `2 ** 3 + 1` parses as `(2 ** 3) + 1` and `-2 ** 2`
//     parses as `-(2 ** 2)`, matching common exponentiation convention).
//
// Merged with the calc base (internal/authoringengine.MergeGrammarJSON) and
// parsed against initialSample, the resulting tree contains both a rule calc
// already had ("expression"/"number") and the delta-added rule ("power") —
// this is the exact "prove the merge took effect" shape
// cmd/verify-authoring-browser asserts on.
func buildInitialDeltaJSON() string {
	scratch := grammargen.CalcGrammar() // fresh Grammar; safe to mutate
	grammargen.AppendChoice(scratch, "expression", grammargen.Sym("power"))
	scratch.Define("power", grammargen.PrecRight(4, grammargen.Seq(
		grammargen.Field("left", grammargen.Sym("expression")),
		grammargen.Field("operator", grammargen.Str("**")),
		grammargen.Field("right", grammargen.Sym("expression")),
	)))

	delta := grammargen.NewGrammar("") // deltas don't need their own name
	delta.Define("expression", scratch.Rules["expression"])
	delta.Define("power", scratch.Rules["power"])

	data, err := grammargen.ExportGrammarJSON(delta)
	if err != nil {
		// Only reachable if grammargen.ExportGrammarJSON itself is broken —
		// every rule here is built from exported DSL helpers, not
		// hand-written JSON, so there is nothing for a caller to usefully
		// recover from; fail loudly at server start rather than serve a
		// broken seed delta.
		panic("authoring: build initial delta grammar.json: " + err.Error())
	}
	return string(data)
}

var initialDeltaJSON = buildInitialDeltaJSON()

// initialSample exercises both the inherited calc base (the "+", "number")
// and the delta-added "power" rule in one parse, so the default page load
// already shows the inheritance story end to end.
const initialSample = `2 ** 3 + 1`

func init() {
	docsapp.RegisterStaticDocsPage(
		"Grammar Authoring",
		"Inherit a base grammar and author a delta of added/overridden rules in the browser — grammargen merges and compiles the extended grammar live.",
		route.FileModuleOptions{
			Load: loadAuthoring,
			Metadata: func(ctx *route.RouteContext, page route.FilePage, data any) (server.Metadata, error) {
				return server.Metadata{
					Title: server.Title{Absolute: "Grammar Authoring — gotreesitter"},
					Links: []server.LinkTag{{
						Rel:  "stylesheet",
						Href: docsapp.PublicAssetURL("authoring/authoring.css"),
					}},
				}, nil
			},
		},
	)
}

func loadAuthoring(_ *route.RouteContext, _ route.FilePage) (any, error) {
	return map[string]any{
		"grammar":    initialDeltaJSON,
		"sample":     initialSample,
		"gtsVersion": docsapp.PlaygroundGTSVersion(),
		"wasmURL":    docsapp.PublicAssetURL("authoring/authoring.wasm"),
		// workerURL points at the generated Web Worker bootstrap
		// (cmd/build-authoring-wasm writes it alongside authoring-worker.wasm)
		// that actually runs the compile+parse+diagnose+highlight loop off
		// the main thread. See cmd/authoring-wasm/main_js.go's package doc.
		"workerURL": docsapp.PublicAssetURL("authoring/authoring-worker.js"),
		// baseIndexURL points at the Phase 2 inheritance base catalog
		// (cmd/build-authoring-wasm's writeBaseAssets writes it alongside
		// public/authoring/bases/<name>.grammar.json). The base picker
		// (#ag-base in page.gsx) fetches it at mount.
		"baseIndexURL": docsapp.PublicAssetURL("authoring/bases/index.json"),
	}, nil
}
