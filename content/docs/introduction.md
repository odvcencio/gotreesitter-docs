---
title: Introduction
description: gotreesitter is a pure-Go, byte-exact reimplementation of tree-sitter — no CGo, no C toolchain, 206 grammars built in.
nav_group: Introduction
order: 1
---

**gotreesitter** is a pure-Go reimplementation of [tree-sitter](https://tree-sitter.github.io/), an
incremental parsing system. gotreesitter uses the same grammar format, the same parse-table
approach, and the same incremental-reparse model as tree-sitter. The runtime contains zero C code.

## What it is, and isn't

gotreesitter is not a CGo binding to the C tree-sitter library. It is a from-scratch
reimplementation of the runtime: the lexer, the LR/GLR parser, the incremental engine, the arena
allocator, the query engine, and the tree cursor are all written in Go. No code is translated or
copied from the C implementation.

gotreesitter shares one thing with upstream tree-sitter: the input format. gotreesitter reads the
same `grammar.json` file that `tree-sitter generate` produces. Its own tool, `ts2go`, extracts
parse tables directly from upstream `parser.c` files. The `grammars` package ships 206
pre-compiled grammars. Each grammar comes from its real upstream repository; none are
hand-approximated.

This double effort has a payoff. gotreesitter produces syntax trees that are byte-exact matches
against the C runtime (tree-sitter v0.25.0), wherever the two implementations have been compared.
Its error-recovery engine is checked decision-by-decision against that same C runtime, for every
language that has gone through the process (see below).

The relationship to [upstream tree-sitter](https://tree-sitter.github.io/tree-sitter/) is
independent implementation with a shared interface. gotreesitter reads the same `grammar.json`
files and the same query language as upstream, so grammars and queries written for the C
ecosystem carry over unchanged. Upstream's documentation is canonical for the parts both
implementations share: grammar-writing craft and the query-language spec. These pages link to
that documentation instead of restating it. What these pages document is the Go engine itself,
and every place where its behavior or API differs from upstream.

## Why it exists

Every other Go tree-sitter binding wraps the C library through CGo. That approach has a cost that
only shows up when you try to ship the software:

- Cross-compiling needs a C cross-toolchain for the target. A build with `GOOS=wasip1`, an
  unusual `GOARCH`, or a Windows target without MSYS2/MinGW fails to link.
- CI images need `gcc` plus the grammar's C sources. `go install` breaks for anyone without a C
  compiler on their machine.
- The Go race detector, fuzzer, and coverage tooling cannot see across the CGo boundary. Bugs in
  the C runtime or the FFI marshaling stay invisible to `go test -race`.

A Go program that wanted real tree-sitter parsing used to pay one of these costs, or do without
parsing at all. gotreesitter removes the C dependency instead of hiding it: run `go get`, then
build a single static binary for any target Go supports.

## What you get

- **206 embedded grammars.** There is no separate install step, and no `.so` or `.wasm` file to
  fetch at runtime.
- **206/206 curated structural parity** against the pinned C oracle. No known-degraded structural
  skips remain, as of v0.23.0.
- **A single static binary.** `go build` is the whole pipeline. There is no C toolchain to
  provision in CI or on a teammate's machine.
- **Byte-exact syntax trees**, verified against the C runtime where checked.
- **Oracle-gated recovery and ambiguity handling.** Curated and real-corpus suites compare the
  selected Go tree with a pinned C runtime. The suites report correctness and performance
  separately.
- **Incremental parsing that is orders of magnitude faster than a full parse.** Reparsing after a
  no-op edit runs on the engine's 0 B / 0-allocation hot path and returns in single-digit
  nanoseconds. A real edit still reuses almost the whole tree and finishes in a small fraction of
  the time a full parse takes.
- **Honest full-parse receipts.** The canonical claim is a locked publication receipt on four
  human-authored, genuinely forking Go fixtures: a 4.851050× C equal-fixture geomean against a
  fingerprinted static oracle. The project withdrew the old 1.895× synthetic headline as
  unrepresentative. The current fleet median is about 3× C, and per-language cliffs and held-outs
  stay visible in the ratcheted ledger.

## The honest asterisks

This project states where it still falls short, rather than let you find that out in production.

- **Peak memory per node is higher than C's.** The Go node header has fallen from 144 to 104
  bytes, and the Poppler retained heap is cut by 52.5%. The pointer-rich tree still costs more
  than C's compact representation.
- **A handful of very large, generated files can still hit the default memory budget.** The
  asm.js-class extreme of a JS/TS corpus is one example.
- **The full-parse ratchet covers 204 of 206 grammars.** D and F# remain explicit held-outs. 101
  rows in the 2026-07-11 ledger were above 3× C.
- **Poppler correctness and hard-containment are banked, but ordinary-budget economy is not.**
  The 3.4 MB JavaScript witness reaches exact parity inside a hard 2 GiB container. The normal
  512 MiB policy still stops the parse, and its successful full parse remains 3.50× C.

The discipline behind these claims is the same discipline that finds them: compile C tree-sitter
v0.25.0 with printf instrumentation, replay the Go parser's decisions against it one at a time,
and fix every divergence until the trees are byte-exact. That process keeps turning up
"performance cliffs" that turn out to be correctness bugs wearing a performance costume. One bash
file took 46.4 seconds and produced a whole-file error tree; the parser was silently evicting the
correct parse lineage. After the fix, the same file parses in about 8 ms, byte-exact. The same
pattern — slow because wrong, not slow because hard — has repeated in PHP, Rust, and Kotlin. A
profiler shows you where time goes. Only an oracle comparison shows you that the timed work
should not have happened at all.

## Who it's for

If you are building an editor, a linter, an LSP, or a code-intelligence index — anything that
walks real syntax trees at scale — gotreesitter gives you that without asking your users to
install a C toolchain first. It also suits one-off batch analysis over a large codebase, where
incremental reparsing does not matter but 206 ready-to-use grammars and a single binary do.

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
