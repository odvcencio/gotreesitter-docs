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
// landing page (app/page.gsx): tok/ttl/body/color in display order.
var landingFeatures = []map[string]any{
	{"tok": "noC", "ttl": "No CGo, no C toolchain", "body": "Parser, lexer, scanners, query engine — all Go. Nothing to link, nothing to install.", "color": "c-cyan"},
	{"tok": "→*", "ttl": "Cross-compiles anywhere", "body": "Any GOOS/GOARCH Go supports, wasip1 included. No per-target C cross-toolchain.", "color": "c-blue"},
	{"tok": "206", "ttl": "206 grammars in the box", "body": "Extracted from upstream parser.c by ts2go, compressed to blobs, lazy-loaded with an LRU cache.", "color": "c-violet"},
	{"tok": "1.9×", "ttl": "Measured full parses", "body": "The corrected materialized Go benchmark is 1.895× C on the pinned host. Fleet ratios and caveats are published per language.", "color": "c-green"},
	{"tok": "ns", "ttl": "Zero-allocation incrementals", "body": "The pinned-host no-edit path is ~9.9 ns; the one-byte edit is ~1.98 µs. Both allocate zero.", "color": "c-orange"},
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
