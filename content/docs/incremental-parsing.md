---
title: Incremental Parsing
description: Re-parse only what changed after an edit, reusing unchanged subtrees instead of parsing the whole file again.
nav_group: Using the Parser
order: 4
---

An editor re-parses on every keystroke. If parsing a large file from scratch takes a millisecond, and a user types at ten keystrokes a second, a naive editor integration spends ten milliseconds a second just re-deriving syntax it already knew. Incremental parsing is how tree-sitter (and gotreesitter) avoid that: you tell the parser exactly what byte range changed, and it reuses every subtree that the edit didn't touch, re-lexing and re-parsing only the invalidated span.

On the pinned-host benchmark (a generated 500-function, 19,294-byte Go file,
`GOMAXPROCS=1`, median of 10 runs):

| Parse | Median time | Allocs/op |
|---|---:|---:|
| Full parse, materialized | 10.907 ms | 9 |
| Incremental, one byte edited | 1.98 µs | 0 |
| Incremental, no edit | 9.9 ns | 0 |

A single-byte edit is about **5,500× faster** than a full parse of the same file; a no-op check
is about **1.1 million× faster**. Both incremental lanes allocate zero. (Earlier releases also
published speedup multipliers against the cgo binding; those calibration rows were withdrawn with
the old full-parse headline because the binding used a mismatched grammar.) Full figures and
methodology live in the project's canonical
[`BENCH.md`](https://github.com/odvcencio/gotreesitter/blob/v0.37.0/BENCH.md).

This page assumes you already have a `*gotreesitter.Tree` from a first `parser.Parse` call — see [Syntax Trees and Nodes](/docs/syntax-trees-and-nodes) if you need that first, or [Tree Cursors](/docs/tree-cursors) for traversal patterns that keep working across reparses.

## The workflow

1. Parse once and keep the tree.
2. On each edit: record it on the tree with `tree.Edit(InputEdit{...})`.
3. Re-parse with `parser.ParseIncremental(newSource, oldTree)`.

```go
tree, err := parser.Parse(src)
if err != nil {
    log.Fatal(err)
}

// ... user edits src to produce newSrc ...

tree.Edit(gts.InputEdit{
    StartByte:   42,
    OldEndByte:  42,
    NewEndByte:  43,
    StartPoint:  gts.Point{Row: 3, Column: 10},
    OldEndPoint: gts.Point{Row: 3, Column: 10},
    NewEndPoint: gts.Point{Row: 3, Column: 11},
})

newTree, err := parser.ParseIncremental(newSrc, tree)
```

`tree.Edit` does not re-parse anything by itself. It shifts the byte offsets and points of every node after the edit, and marks nodes that overlap the edited region as dirty, so `ParseIncremental` knows what it can trust and what it has to re-derive. You can call `tree.Edit` more than once before reparsing — each call is recorded (`tree.Edits() []InputEdit`), and the shifts compose correctly.

## `InputEdit`

```go
type InputEdit struct {
    StartByte   uint32
    OldEndByte  uint32
    NewEndByte  uint32
    StartPoint  Point
    OldEndPoint Point
    NewEndPoint Point
}
```

Six fields, describing the edit as a byte-range replacement plus its row/column ("point") equivalents:

- **`StartByte`** — where the edit begins, in the *old* source.
- **`OldEndByte`** — where the replaced span ends, in the *old* source.
- **`NewEndByte`** — where the replacement ends, in the *new* source.
- **`StartPoint` / `OldEndPoint` / `NewEndPoint`** — the same three positions as `{Row, Column}` (both zero-based; `Column` is a byte offset within the row).

The three byte fields tell you what kind of edit it was:

| Edit | Byte pattern |
|---|---|
| Pure insertion | `StartByte == OldEndByte`, `NewEndByte > StartByte` |
| Pure deletion | `NewEndByte == StartByte`, `OldEndByte > StartByte` |
| Replacement | `OldEndByte > StartByte` and `NewEndByte > StartByte`, generally unequal |

To compute one from an edit you already know the coordinates for (the common editor case): `StartByte`/`StartPoint` are the edit's start position in the old buffer, `OldEndByte`/`OldEndPoint` are where the replaced text used to end, and `NewEndByte`/`NewEndPoint` are where the replacement text now ends — all still expressed against `StartPoint`, i.e. you need to walk the replacement text to know how many rows/columns it spans. If your editor gives you UTF-16 code-unit offsets (LSP does), don't hand-roll this: `Tree.InputEditForUTF16` and `Tree.EditUTF16` convert and apply a `UTF16Edit` directly — see the UTF-16 section of the README.

`Node.Edit(edit)` also exists, adjusting a single node's span from whatever root contains it, but it doesn't record edit history — prefer `Tree.Edit` unless you're doing something bespoke.

## What `ParseIncremental` reuses

```go
func (p *Parser) ParseIncremental(source []byte, oldTree *Tree) (*Tree, error)
```

`ParseIncremental` walks the old tree's spine, identifies the region actually invalidated by the recorded edit(s), and reuses everything outside it by reference — both leaf and non-leaf subtrees are eligible; non-leaf reuse is driven by pre-goto state tracking on interior nodes, so the parser can skip an entire unchanged subtree (a whole function body, say) without re-deriving its contents node by node. Only the invalidated span is re-lexed and re-parsed, and the result is stitched back together with the reused subtrees around it.

When no edit was recorded at all — `source` is byte-identical to `oldTree`'s source and no `Edit`
call happened — `ParseIncremental` returns `oldTree` itself on a pointer check, in single-digit
nanoseconds with zero allocations (9.9 ns on the pinned receipt). This makes it cheap to call
`ParseIncremental` speculatively rather than tracking "did anything actually change" yourself.

UTF-16, custom token sources, and profiling all have incremental counterparts: `ParseIncrementalUTF16`, `ParseIncrementalWithTokenSource`, `ParseIncrementalProfiled` (returns an `IncrementalParseProfile` with reuse attribution), and the option-based `parser.ParseWith(source, gotreesitter.WithOldTree(oldTree))`.

## What changed: two different questions

Once you have `newTree`, you often need to know *which byte ranges actually changed* — to re-run a highlighter only over the affected span, invalidate a diagnostics cache, or similar. There are two APIs here that answer different questions:

- **`tree.ChangedRanges() []Range`** — the edited tree's own bookkeeping. It returns the byte/point ranges from the `InputEdit`s you recorded on that tree via `Edit`, coalesced where they overlap. This is cheap (no second tree needed) but literal: it reports exactly the spans you told it about, not what the reparse actually changed structurally.
- **`gotreesitter.DiffChangedRanges(oldTree, newTree *Tree) []Range`** — a structural diff between the edited old tree and the freshly reparsed new tree (equivalent to C tree-sitter's `ts_tree_get_changed_ranges`). It walks both trees together and reports the minimal ranges where syntax actually differs, which is frequently *larger* than the literal edit: inserting one character can change the token boundaries of everything after it on the same line.

For "what should I re-highlight," `DiffChangedRanges` is almost always the one you want; `tree.ChangedRanges()` is a lighter-weight edit log for callers that already track their own edit spans.

## Compile-checked example

```go
package main

import (
	"bytes"
	"fmt"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func main() {
	lang := grammars.GoLanguage()
	parser := gts.NewParser(lang)

	src := []byte("package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n")

	tree, err := parser.Parse(src)
	if err != nil {
		panic(err)
	}
	defer tree.Release()

	// User types "x" right after "return a", turning it into "return ax".
	needle := []byte("return a")
	insertAt := uint32(bytes.Index(src, needle) + len(needle))

	newSrc := append([]byte{}, src[:insertAt]...)
	newSrc = append(newSrc, 'x')
	newSrc = append(newSrc, src[insertAt:]...)

	tree.Edit(gts.InputEdit{
		StartByte:   insertAt,
		OldEndByte:  insertAt,
		NewEndByte:  insertAt + 1,
		StartPoint:  gts.Point{Row: 3, Column: 9},
		OldEndPoint: gts.Point{Row: 3, Column: 9},
		NewEndPoint: gts.Point{Row: 3, Column: 10},
	})

	newTree, err := parser.ParseIncremental(newSrc, tree)
	if err != nil {
		panic(err)
	}
	defer newTree.Release()

	fmt.Println("tree.ChangedRanges (literal edit span):")
	for _, r := range tree.ChangedRanges() {
		fmt.Printf("  [%d, %d) = %q\n", r.StartByte, r.EndByte, newSrc[r.StartByte:r.EndByte])
	}

	fmt.Println("DiffChangedRanges (structural diff):")
	for _, r := range gts.DiffChangedRanges(tree, newTree) {
		fmt.Printf("  [%d, %d) = %q\n", r.StartByte, r.EndByte, newSrc[r.StartByte:r.EndByte])
	}
}
```

This prints:

```text
tree.ChangedRanges (literal edit span):
  [48, 49) = "x"
DiffChangedRanges (structural diff):
  [40, 53) = "return ax + b"
```

One inserted byte, but the identifier `a` became `ax`, which changed the shape of the surrounding binary expression — `DiffChangedRanges` reports the full `return ax + b` span that a highlighter or analysis pass actually needs to revisit, while `tree.ChangedRanges()` reports only the literal one-byte insertion you recorded.

## Concurrency

A `*Parser` is **not** safe for concurrent use — use one parser per goroutine, or a `ParserPool` (`gotreesitter.NewParserPool(lang)`), which checks out a scrubbed `*Parser` per call and is safe to share. For batch indexers, the pool is also where you bound worst-case inputs: `NewParserPool(lang, gotreesitter.WithParserPoolTimeoutMicros(5_000_000))` caps any single parse at five seconds, so one pathological file can't stall a whole indexing run. This is a load-bearing pattern in the wild — external symbol indexers built on gotreesitter share one timeout-bounded pool per language across workers, and pair it with tree reuse: parse once, then hand the same `*Tree` to queries, metrics, and extraction instead of re-parsing per concern. `*Tree` is safe for **concurrent reads** after construction, but `Edit` and `Release` are not — don't call `tree.Edit` from one goroutine while another reads the same tree. If you hold a [`TreeCursor`](/docs/tree-cursors) across an edit or a reparse, recreate it afterward: cursors hold direct pointers into tree nodes that an edit or incremental reparse can invalidate.

Queries compose directly with this: compile a `*Query` once and re-run it against each `newTree` returned by `ParseIncremental` — see [Queries](/docs/queries) for the query engine, or restrict a re-run to just the changed span with `QueryCursor.SetByteRange` fed from `DiffChangedRanges`.
