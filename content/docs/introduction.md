---
title: Introduction
description: gotreesitter is a pure-Go, byte-exact reimplementation of tree-sitter — no CGo, no C toolchain, 206 grammars built in.
nav_group: Introduction
order: 1
---

**gotreesitter** is a pure-Go reimplementation of the [tree-sitter](https://tree-sitter.github.io/)
incremental parsing system: the same grammar format, the same parse-table approach, the same
incremental-reparse model — with zero C in the runtime.

## What it is, and isn't

gotreesitter is not a CGo binding to the C tree-sitter library. It's a from-scratch
reimplementation of the runtime — lexer, LR/GLR parser, incremental engine, arena allocator,
query engine, tree cursor — all in Go. No code is translated or copied from the C implementation.

What it shares with upstream tree-sitter is the input format. gotreesitter consumes the same
`grammar.json` that `tree-sitter generate` produces, and its own tool (`ts2go`) extracts parse
tables directly from upstream `parser.c` files. 206 grammars ship pre-compiled in the `grammars`
package, each pulled from the grammar's real upstream repository, not hand-approximated.

The payoff of doing the work twice: parse trees that are byte-exact matches against the C runtime
(tree-sitter v0.25.0) wherever the two have been compared, and an error-recovery engine checked
decision-by-decision against that same C runtime for every language that has gone through the
process (more below).

The relationship to [upstream tree-sitter](https://tree-sitter.github.io/tree-sitter/), then, is
independent implementation, shared interface: gotreesitter consumes the same `grammar.json` files
and the same query language, so grammars and queries written for the C ecosystem carry over. For
the parts both implementations share — grammar-writing craft, the query-language spec — upstream's
documentation is canonical, and these pages link it rather than restating it; what's documented
here is the Go engine itself and every place its behavior or API differs.

## Why it exists

Every other Go tree-sitter binding wraps the C library through CGo, and that has a cost that only
shows up once you try to ship:

- Cross-compiling needs a C cross-toolchain for the target. `GOOS=wasip1`, an unusual `GOARCH`, or
  a Windows build without MSYS2/MinGW simply fails to link.
- CI images need `gcc` plus the grammar's C sources, and `go install` breaks for anyone without a C
  compiler on their machine.
- The Go race detector, fuzzer, and coverage tooling can't see across the CGo boundary, so bugs in
  the C runtime or the FFI marshaling are invisible to `go test -race`.

A Go program that wanted real tree-sitter parsing used to pay one of these taxes, or do without.
gotreesitter removes the C dependency instead of hiding it: `go get`, then a single static binary,
for any target Go supports.

## What you get

- **206 embedded grammars** — no separate install step, no fetching `.so` or `.wasm` files at
  runtime.
- **206/206 curated structural parity** against the pinned C oracle, with no
  known-degraded structural skips as of v0.23.0.
- **A single static binary.** `go build` is the whole pipeline; there's no C toolchain to
  provision in CI or on a teammate's machine.
- **Byte-exact syntax trees**, verified against the C runtime where checked.
- **C-faithful error recovery** for the \~124 grammars elected into the oracle campaign so far — the
  parser's recovery decisions match C tree-sitter's, not an approximation of them. The rest use a
  resync-based recovery path today and are staged for election.
- **Incremental parsing that is orders of magnitude faster than a full parse.** Reparsing after a
  no-op edit runs on the engine's 0 B / 0 allocation hot path and returns in single-digit
  nanoseconds; a real edit still reuses almost the whole tree and finishes in a small fraction of a
  full parse.
- **Full-parse performance at or near C** for the coding languages that matter most, with a
  scaling curve that favors Go as files grow — a measured scaling exponent of \~1.20 versus C's
  \~1.29 on tested languages.

## The honest asterisks

This project would rather tell you where it's still catching up than let you find out in
production.

- **Peak memory per node is higher than C's** — roughly 152 bytes/node in gotreesitter versus C's
  16–64 bytes/node. That's a structural gap between Go's node representation and C's, not a bug,
  and it's tracked rather than hidden.
- **A handful of very large, generated files** — the asm.js-class extreme of a JS/TS corpus, for
  instance — can still hit the default memory budget.
- **The full-parse ratchet covers 203/206 grammars**, but coverage is not the same
  as near-C speed: several measured rows remain over the ≤2× target, while D and F#
  remain explicit held-outs and Groovy uses a named scoped basis.
- **The JavaScript Poppler-scale memory cliff remains open.** It is recorded in the
  Wave 3 ledger rather than being presented as a green performance measurement.

The discipline behind these claims is the same one that finds them: compile C tree-sitter v0.25.0
with printf instrumentation, replay the Go parser's decisions against it one at a time, and fix
every divergence until the trees are byte-exact. That process keeps turning up "performance
cliffs" that were actually correctness bugs wearing a performance costume. A bash file that took
46.4 seconds and produced a whole-file error tree turned out to be the parser silently evicting
the correct parse lineage; fixed, the same file parses in \~8 ms, byte-exact. The same pattern —
slow because wrong, not slow because hard — has repeated in PHP, Rust, and Kotlin. A profiler
tells you where time goes; only an oracle comparison tells you the work being timed shouldn't have
happened at all.

## Who it's for

If you're building an editor, a linter, an LSP, or a code-intelligence index — anything that walks
real syntax trees at scale — gotreesitter gives you that without asking your users to install a C
toolchain first. It's equally suited to one-off batch analysis over a large codebase, where
incremental reparsing doesn't matter but 206 ready-to-use grammars and a single binary do.

## A quick look

```go title=main.go
package main

import (
	"fmt"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func main() {
	src := []byte(`package main

func main() {
	println("hello, world")
}
`)

	lang := grammars.GoLanguage()
	tree, err := gts.NewParser(lang).Parse(src)
	if err != nil {
		panic(err)
	}

	fmt.Println(tree.RootNode().SExpr(lang))
}
```

```text
(source_file (package_clause (package_identifier)) (function_declaration (identifier) (parameter_list) (block (statement_list (expression_statement (call_expression (identifier) (argument_list (interpreted_string_literal (interpreted_string_literal_content)))))))))
```

> [!TIP] Next
> Ready to write your own parser? Continue to [Getting Started](/docs/getting-started).
