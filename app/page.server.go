package docs

import "m31labs.dev/gosx/route"

func init() {
	RegisterStaticDocsPage(
		"Overview",
		"A pure-Go, byte-exact reimplementation of tree-sitter — no CGo, no C toolchain, 206 grammars built in.",
		route.FileModuleOptions{},
	)
}
