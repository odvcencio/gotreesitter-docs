---
title: Introduction
description: gotreesitter is a pure-Go tree-sitter runtime with 206 built-in grammars and a documented, finite parity scope.
nav_group: Introduction
order: 1
---

gotreesitter is a Go runtime for [tree-sitter](https://tree-sitter.github.io/). It reads
tree-sitter grammar tables and parses source into syntax trees, with incremental parsing, queries,
and language injection. The runtime is implemented in Go and has no CGo dependency.

## What it is

The parser, lexer, incremental engine, arena allocator, query engine, and tree cursor are Go code.
The recovery algorithm follows tree-sitter C's decision steps and cost constants. Differential
tests compare Go trees with a pinned C runtime.

The runtime reads tree-sitter parse tables. ts2go extracts tables from upstream parser.c files,
and grammars ships 206 generated grammar blobs. Those grammar blobs come from their upstream
grammar repositories. The v0.55.1
[README](https://github.com/odvcencio/gotreesitter/blob/v0.55.1/README.md) describes the current
runtime and registry.

## What you can build

- **Parse with built-in grammars.** The v0.55.1 registry contains 206 grammars. Its
  [language guide](https://github.com/odvcencio/gotreesitter/blob/v0.55.1/docs/languages.md)
  describes scanner coverage and the limits of smoke tests.
- **Check trees against C.** The project has structural, query, highlighting, recovery, and
  real-corpus comparisons. Each board has its own tested inputs and date.
- **Reuse unchanged syntax after edits.** The current quiet-host control measured full parsing,
  a one-byte edit, and no-edit reuse on a generated Go file with 500 functions. That file never
  forks, so the result is a control, not typical source code.
- **Use queries and editor features.** The runtime includes query execution, syntax highlighting,
  code navigation, injections, and UTF-16 positions.

Version v0.55.1 uses production GLR parsing by default. Set
GTS_ADMISSION_CANDIDATE=1 to try compact parsing; the language allowlist is empty.

## Measured control input

Upstream measured the exact v0.55.1 source, commit
[92db945f](https://github.com/odvcencio/gotreesitter/commit/92db945f28de67be51de8235c9cd4e25a900f648),
on 2026-09-27 at gts-bench-1: C3-standard-8, Xeon Platinum 8481C, four cores with SMT off,
Ubuntu 24.04, Go 1.26.4, pinned to CPU 2. The medians used 20 shuffled seeds, one process per
seed, GOMAXPROCS=1, -benchtime=750ms, and -benchmem. Upstream's
[benchmark receipt](https://github.com/odvcencio/gotreesitter/blob/main/BENCH.md#primary-trio-baseline)
and [PR #1355](https://github.com/odvcencio/gotreesitter/pull/1355) give the method.

| Benchmark | Median | Allocations/op |
|---|---:|---:|
| Full parse | 8,686,195 ns/op | 8 |
| Single-byte edit | 177,281 ns/op | 5 |
| No-edit reparse | 8.318 ns/op | 0 |

The allocation counts are from the v0.55.1
[CHANGELOG measurement scope](https://github.com/odvcencio/gotreesitter/blob/v0.55.1/CHANGELOG.md#measurement-scope).
The workload is a generated 500-function Go file that never forks. Treat it as a control, not
typical code.

## Parity has a measured scope

The recovery board published on 2026-09-19 records agreement on 39 of 79 cases. Other parity
areas have separate boards and results. A clean result on one fixture does not establish a match
for every input. See [Recovery and Correctness](/docs/recovery-and-correctness) for the board
dates, methods, and known gaps.

The CGo parity harness uses C as a test oracle; it is not linked into the runtime. See
[Contributing](/docs/contributing) for the separate harness and its requirements.

## v1.0 performance goals

The [v1 design](https://github.com/odvcencio/gotreesitter/blob/main/docs/v1-design.md) sets an
engine-floor target of a median no more than 3x C across all 206 grammars, with no file above 5x.
For the top 50, the tuned target is a median no more than 2.5x C, no language above 4x, and
99th-percentile edit latency no more than 20 ms at 137 KiB. Owner decision O-Q3 makes both rows
release-blocking for v1.0. The top-20 stretch target is full parsing at no more than 2x C and edit
latency at or below C. These are targets, not current results. See [The road to v1](/docs/v1).

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
> Continue to [Getting Started](/docs/getting-started) for short steps you can run in your own
> module.
