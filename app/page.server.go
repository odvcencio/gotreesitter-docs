package docs

import "m31labs.dev/gosx/route"

func init() {
	RegisterStaticDocsPage(
		"Overview",
		"A pure-Go, byte-exact reimplementation of tree-sitter — no CGo, no C toolchain, 206 grammars built in.",
		route.FileModuleOptions{
			Bindings: func(ctx *route.RouteContext, page route.FilePage, data any) route.FileTemplateBindings {
				return route.FileTemplateBindings{
					Values: map[string]any{
						"features":   landingFeatures,
						"langTeaser": landingLangTeaser,
					},
				}
			},
		},
	)
}

// landingFeatures backs the "A whole parsing toolkit" `.grid3` on the
// landing page (app/page.gsx). Ported verbatim from
// design/GoTreeSitter-Docs.html's `featureDefs` — tok/ttl/body/color in the
// design's own field names and order.
var landingFeatures = []map[string]any{
	{"tok": "noC", "ttl": "No CGo, no C toolchain", "body": "Parser, lexer, scanners, query engine — all Go. Nothing to link, nothing to install.", "color": "c-cyan"},
	{"tok": "→*", "ttl": "Cross-compiles anywhere", "body": "Any GOOS/GOARCH Go supports, wasip1 included. No per-target C cross-toolchain.", "color": "c-blue"},
	{"tok": "206", "ttl": "206 grammars in the box", "body": "Extracted from upstream parser.c by ts2go, compressed to blobs, lazy-loaded with an LRU cache.", "color": "c-violet"},
	{"tok": "1.5", "ttl": "Beats C on full parse", "body": "1.54 ms vs native C's 1.76 ms on the 500-function Go workload — ~1.15× faster.", "color": "c-green"},
	{"tok": "ns", "ttl": "Nanosecond incrementals", "body": "No-edit reparse is a pointer check: ~2.43 ns, 0 allocations. Single-byte edits ~649 ns.", "color": "c-orange"},
	{"tok": "GLR", "ttl": "C-faithful GLR recovery", "body": "Generalized GLR core reproduces C tree-sitter's recovery decisions for 123 elected grammars.", "color": "c-red"},
	{"tok": "U16", "ttl": "Native UTF-16 for editors", "body": "Parse UTF-16 code units or endian byte buffers; nodes, edits & queries map back to UTF-16 offsets.", "color": "c-pink"},
	{"tok": "gen", "ttl": "Typed query codegen", "body": "tsquery turns .scm files into type-safe Go structs — one per pattern, fully typed captures.", "color": "c-yellow"},
	{"tok": "{ }", "ttl": "Injection parsing", "body": "Parse HTML+JS+CSS, Markdown fences, Vue/Svelte templates. Static & dynamic, recursive, incremental.", "color": "c-violet"},
	{"tok": "rw", "ttl": "Atomic source rewriter", "body": "Collect replace/insert/delete edits, apply in one pass, get InputEdit records for incremental reparse.", "color": "c-blue"},
	{"tok": "+L", "ttl": "Add languages, no fork", "body": "grammar.json → blob → LoadLanguage / RegisterExtension. Ship a grammar as its own Go module.", "color": "c-green"},
	{"tok": "race", "ttl": "Visible to -race", "body": "No CGo boundary means the race detector, coverage, and fuzzer see the entire runtime.", "color": "c-cyan"},
}

// landingLangTeaser backs the "206 grammars, embedded" `.langteaser` chip
// row. Ported verbatim from design/GoTreeSitter-Docs.html's `teaser` list.
var landingLangTeaser = []string{
	"go", "rust", "python", "typescript", "cobol", "zig", "swift", "haskell",
	"cpp", "ruby", "kotlin", "elixir", "nix", "wgsl", "solidity", "ocaml",
}
