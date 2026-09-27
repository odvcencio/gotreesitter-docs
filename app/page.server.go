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
						"features":   landingFeatures,
						"langTeaser": landingLangTeaser,
					},
					Funcs: map[string]any{
						"heroCode": heroCodeNode,
					},
				}
			},
		},
	)
}

// heroSnippet is the hero `.codebody` sample on the landing page
// (app/page.gsx) — the same parse/walk/print shape used throughout the
// docs' own README-style examples.
const heroSnippet = `lang := grammars.GoLanguage()
parser := gotreesitter.NewParser(lang)
tree, err := parser.Parse(src)
if err != nil { panic(err) }
defer tree.Release()
fmt.Println(tree.RootNode().SExpr(lang))

// (source_file (function_declaration ...))`

// heroCodeNode runs heroSnippet through the same gotreesitter-backed
// highlighter every markdown ```go fence uses (highlightSource, in
// highlight.go → renderCodeBlock's convention in render_blocks.go), so the
// hero sample gets real `tk-*` syntax highlighting instead of the plain,
// unhighlighted literal it used to be. Falls back to plain text — never a
// render error — if highlighting can't classify anything.
func heroCodeNode() gosx.Node {
	if highlighted, ok := highlightSource("go", heroSnippet); ok {
		return gosx.RawHTML(highlighted)
	}
	return gosx.Text(heroSnippet)
}

// landingFeatures backs the "A whole parsing toolkit" `.grid3` on the
// landing page (app/page.gsx): tok/ttl/body/color in display order.
var landingFeatures = []map[string]any{
	{"tok": "noC", "ttl": "No CGo, no C toolchain", "body": "Parser, lexer, scanners, query engine — all Go. Nothing to link, nothing to install.", "color": "c-cyan"},
	{"tok": "→*", "ttl": "Cross-compiles anywhere", "body": "Any GOOS/GOARCH Go supports, wasip1 included. No per-target C cross-toolchain.", "color": "c-blue"},
	{"tok": "206", "ttl": "206 grammars in the box", "body": "Grammar tables load on demand; configured cache limits can evict unused grammars.", "color": "c-violet"},
	{"tok": "route", "ttl": "Production route default", "body": "Version v0.55.1 uses production GLR parsing by default. Set GTS_ADMISSION_CANDIDATE=1 to enable compact parsing. Compact parser graduation remains incomplete.", "color": "c-green"},
	{"tok": "ns", "ttl": "Edit and no-edit control", "body": "The 2026-09-27 control on a generated Go file measured 177,281 ns for a single-byte edit and 8.318 ns for no-edit reuse. The file never forks, so this is a control, not typical code. See the performance page for the source, host, and method.", "color": "c-orange"},
	{"tok": "GLR", "ttl": "C-oracle recovery gates", "body": "The GLR and recovery paths are verified separately from performance, with curated and real-corpus parity receipts.", "color": "c-red"},
	{"tok": "U16", "ttl": "Native UTF-16 for editors", "body": "Parse UTF-16 code units or endian byte buffers; nodes, edits & queries map back to UTF-16 offsets.", "color": "c-pink"},
	{"tok": "gen", "ttl": "Typed query codegen", "body": "tsquery turns .scm files into type-safe Go structs — one per pattern, fully typed captures.", "color": "c-yellow"},
	{"tok": "{ }", "ttl": "Injection parsing", "body": "Parse HTML+JS+CSS, Markdown fences, Vue/Svelte templates. Static & dynamic, recursive, incremental.", "color": "c-violet"},
	{"tok": "rw", "ttl": "Atomic source rewriter", "body": "Collect replace/insert/delete edits, apply in one pass, get InputEdit records for incremental reparse.", "color": "c-blue"},
	{"tok": "+L", "ttl": "Add languages, no fork", "body": "grammar.json → blob → LoadLanguage / RegisterExtension. Ship a grammar as its own Go module.", "color": "c-green"},
	{"tok": "race", "ttl": "Visible to -race", "body": "No CGo boundary means the race detector, coverage, and fuzzer see the entire runtime.", "color": "c-cyan"},
}

// landingLangTeaser backs the "206 grammars, embedded" `.langteaser` chip row.
var landingLangTeaser = []string{
	"go", "rust", "python", "typescript", "cobol", "zig", "swift", "haskell",
	"cpp", "ruby", "kotlin", "elixir", "nix", "wgsl", "solidity", "ocaml",
}
