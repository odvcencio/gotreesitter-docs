---
title: Incremental Parsing
description: Record an edit, reparse with the previous tree, and preserve tree ownership in a Go editor.
nav_group: Using the Parser
order: 4
---

After an edit, incremental parsing can reuse unchanged parts of the previous tree. You record
the edit on the old tree, then give that tree and the new source to the parser. The parser reuses
subtrees where its safety checks allow and reparses the rest.

The certified route uses a fresh parse when it cannot validate reuse, including
some edits to malformed inputs. Current defaults and fallback conditions are in
[Parser Engines and Upgrade Guide](/docs/parser-engines).

This page assumes you have a tree from `Parser.Parse`.
See [Syntax Trees and Nodes](/docs/syntax-trees-and-nodes) for tree ownership.

## The workflow

1. Parse once and keep the tree.
2. On each edit, record it on the tree with `tree.Edit(InputEdit{...})`.
3. Re-parse with `parser.ParseIncremental(newSource, oldTree)`.

```go
tree, err := parser.Parse(src)
if err != nil {
    tree.Release()
    log.Fatal(err)
}
defer tree.Release()

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
if err != nil {
    newTree.Release()
    log.Fatal(err)
}
defer newTree.Release()
fmt.Println(newTree.RootNode().HasError())
```

`tree.Edit` does not re-parse anything by itself. It shifts the byte offsets and points of every
node after the edit, and it marks nodes that overlap the edited region as dirty, so
`ParseIncremental` knows what it can trust and what it has to re-derive. You can call `tree.Edit`
more than once before reparsing — each call is recorded (`tree.Edits() []InputEdit`), and the
shifts compose correctly.

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

`InputEdit` has six fields. They describe the edit as a byte-range replacement, plus its
row/column ("point") equivalents:

- **`StartByte`** — where the edit begins, in the *old* source.
- **`OldEndByte`** — where the replaced span ends, in the *old* source.
- **`NewEndByte`** — where the replacement ends, in the *new* source.
- **`StartPoint` / `OldEndPoint` / `NewEndPoint`** — the same three positions as `{Row, Column}`
  (both zero-based; `Column` is a byte offset within the row).

The three byte fields tell you what kind of edit it was:

| Edit | Byte pattern |
|---|---|
| Pure insertion | `StartByte == OldEndByte`, `NewEndByte > StartByte` |
| Pure deletion | `NewEndByte == StartByte`, `OldEndByte > StartByte` |
| Replacement | `OldEndByte > StartByte` and `NewEndByte > StartByte`, generally unequal |

To compute one from an edit you already know the coordinates for (the common editor case):
`StartByte`/`StartPoint` are the edit's start position in the old buffer, `OldEndByte`/
`OldEndPoint` are where the replaced text used to end, and `NewEndByte`/`NewEndPoint` are where
the replacement text now ends. All three still express against `StartPoint`, so you need to walk
the replacement text to know how many rows and columns it spans. If your editor gives you UTF-16
code-unit offsets (LSP does), do not hand-roll this conversion: `Tree.InputEditForUTF16` and
`Tree.EditUTF16` convert and apply a `UTF16Edit` directly — see the UTF-16 section of the README.

`Node.Edit(edit)` also exists. It adjusts a single node's span from whatever root contains it, but
it does not record edit history — prefer `Tree.Edit` unless you are doing something bespoke.

## What `ParseIncremental` reuses

```go
var parseIncremental func(parser *gts.Parser, source []byte, oldTree *gts.Tree) (*gts.Tree, error)
```

The parser uses the old tree and edit history to find reusable subtrees.
Unchanged regions can still require work if source positions shift, a scanner's
state needs to be rebuilt, or the parse reaches syntax that depends on the edit.
A parse can also run fresh if the reuse checks decline. Keep the old source
buffer immutable while any tree uses it.

When the source is unchanged, no edit is pending, and included ranges still match,
`ParseIncremental` can return the same tree with an additional retained handle.
Release each returned handle, even when the old and new pointers are equal.
The dated no-edit control result is on [Performance](/docs/performance).

Legacy reparses can update parent links on reused nodes. Read the returned tree
after the reparse. Copy the old tree before editing if a reader needs an
independent snapshot, and release the copy when that reader finishes.
`Tree.Edit` already updates node spans, so apply each edit once.

A complete parse can contain error nodes. Check `RootNode().HasError()` for
syntax and `ParseStopReason()` for early stops. The strict incremental method
returns an error on an early stop. Release a returned tree even when you also
receive an error.

UTF-16, custom token sources, and profiling all have incremental counterparts:
`ParseIncrementalUTF16`, `ParseIncrementalWithTokenSource`, `ParseIncrementalProfiled` (returns an
`IncrementalParseProfile` with reuse attribution), and the option-based
`parser.ParseWith(source, gotreesitter.WithOldTree(oldTree))`.

## What changed: two different questions

Once you have `newTree`, you often need to know *which byte ranges actually changed* — to
re-run a highlighter only over the affected span, invalidate a diagnostics cache, or handle a
similar task. Two APIs answer different questions here:

- **`tree.ChangedRanges() []Range`** — the edited tree's own bookkeeping. It returns the
  byte/point ranges from the `InputEdit`s you recorded on that tree through `Edit`, coalesced
  where they overlap. This call is cheap (it needs no second tree) but literal: it reports
  exactly the spans you told it about, not what the reparse actually changed structurally.
- **`gotreesitter.DiffChangedRanges(oldTree, newTree *Tree) []Range`** — a structural diff between
  the edited old tree and the freshly reparsed new tree (equivalent to C tree-sitter's
  `ts_tree_get_changed_ranges`). It walks both trees together and reports the minimal ranges
  where syntax actually differs, which is frequently *larger* than the literal edit: inserting one
  character can change the token boundaries of everything after it on the same line.

For "what should I re-highlight," reach for `DiffChangedRanges` in almost every case;
`tree.ChangedRanges()` serves as a lighter-weight edit log for callers that already track their
own edit spans.

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

One inserted byte changed the identifier `a` to `ax`, which changed the shape of the surrounding
binary expression. `DiffChangedRanges` reports the full `return ax + b` span that a highlighter or
analysis pass actually needs to revisit, while `tree.ChangedRanges()` reports only the literal
one-byte insertion you recorded.

## Concurrency

A `*Parser` is **not** safe for concurrent use — use one parser per goroutine, or use a
`ParserPool` (`gotreesitter.NewParserPool(lang)`), which checks out a scrubbed `*Parser` per call
and is safe to share. For batch indexers, the pool is also where you bound worst-case inputs:
`NewParserPool(lang, gotreesitter.WithParserPoolTimeoutMicros(5_000_000))` caps any single parse
at five seconds, so one pathological file cannot stall a whole indexing run. This is a
load-bearing pattern in the wild — external symbol indexers built on gotreesitter share one
timeout-bounded pool per language across workers, and they pair it with tree reuse: parse once,
then hand the same `*Tree` to queries, metrics, and extraction instead of re-parsing per concern.
`*Tree` is safe for **concurrent reads** after construction, but `Edit` and `Release` are not —
do not call `tree.Edit` from one goroutine while another reads the same tree. If you hold a
[`TreeCursor`](/docs/tree-cursors) across an edit or a reparse, recreate it afterward: cursors
hold direct pointers into tree nodes, and an edit or incremental reparse can invalidate them.

Queries compose directly with this: compile a `*Query` once and re-run it against each `newTree`
returned by `ParseIncremental` — see [Queries](/docs/queries) for the query engine, or restrict a
re-run to just the changed span with `QueryCursor.SetByteRange` fed from `DiffChangedRanges`.
