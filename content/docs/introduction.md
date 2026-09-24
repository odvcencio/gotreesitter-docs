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
against the C runtime (tree-sitter v0.25.1), on the curated cases that pass the comparison. Other cases have known differences.
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
- **A curated structural gate for 206 grammars** against the pinned C oracle, with no
  allowed known-degraded structural entries. The dated boards describe other tested scopes.
- **A single static binary.** `go build` is the whole pipeline. There is no C toolchain to
  provision in CI or on a teammate's machine.
- **Byte-exact syntax trees**, verified against the C runtime where checked.
- **Oracle-gated recovery and ambiguity handling.** Curated and real-corpus suites compare the
  selected Go tree with a pinned C runtime. The suites report correctness and performance
  separately.
- **Incremental reuse.** The 2026-09-23 generated-Go control measured 439.825 µs and five
  allocations for a single-byte edit. No-edit reuse measured 27.4 ns with zero allocations.
- **Dated full-parse evidence.** The sealed v9 receipt from 2026-08-02 measured 4.815× C for
  production and 3.986× C for compact parsing on four frozen Go files.

Version v0.54.0 makes the production GLR route the default. Set
`GTS_ADMISSION_CANDIDATE=1` to enable compact parsing. Graduation remains incomplete.
The benchmark results above apply to their pinned revisions, not the exact v0.54.0 tag.
See [Performance](/docs/performance) for the source, host, method, and limits.

## Scope

The [tagged README](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/README.md)
reports 206 grammars and 119 Go external scanners. Smoke success does not prove parity for
every input. The project publishes separate boards for queries, highlighting, recovery,
and real-code inputs. See [Recovery and Correctness](/docs/recovery-and-correctness).

The [roadmap](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/docs/roadmap.md)
keeps memory work and compact parser graduation open. It identifies 1.5× C on the locked
real-code matrix as a future target, not a result.

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
