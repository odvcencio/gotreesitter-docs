---
title: Syntax Highlighting
description: Turn source code into styled ranges with the Highlighter — embedded highlight queries for all 206 grammars, incremental re-highlighting, and nested fenced-code highlighting.
nav_group: Using the Parser
order: 5
---

A `Highlighter` bundles a parser, a compiled highlight query, and a language into one call:
give it source bytes, get back a sorted, non-overlapping list of `HighlightRange` values — byte
spans tagged with capture names like `"keyword"` or `"function.name"`. Your editor (or HTML
renderer, or terminal pager) maps those capture names to styles; the highlighter deliberately
knows nothing about colors or themes.

This is the same query-driven model as upstream tree-sitter's
[syntax highlighting](https://tree-sitter.github.io/tree-sitter/3-syntax-highlighting.html),
with one practical difference in packaging: upstream distributes highlight queries as
`queries/highlights.scm` files that a consumer must locate and load, while gotreesitter embeds
the highlight query for every one of its 206 built-in grammars directly in the `grammars`
registry. There is no `.scm` file to find, ship, or version-match — `LangEntry.HighlightQuery`
is a string that's already there.

If you haven't met the query language yet, read [Queries](/docs/queries) first — a highlight
query is an ordinary query whose capture names happen to be style-ish
(`@keyword`, `@string`, `@function.name`), and everything on that page applies here.

## Quick start

```go
entry := grammars.DetectLanguage("sample.md")
lang := entry.Language()

hl, err := gts.NewHighlighter(lang, entry.HighlightQuery)
if err != nil {
    log.Fatal(err)
}

ranges := hl.Highlight([]byte("# Sample\n\n```go\nfor i := 0; i < 3; i++ {\n\tprintln(i)\n}\n```\n"))
for _, r := range ranges {
    fmt.Printf("[%d, %d) %s\n", r.StartByte, r.EndByte, r.Capture)
}
```

Three things worth noticing. `DetectLanguage` (see [Languages](/docs/languages)) picks the
grammar from the file extension and hands back a `*LangEntry`; its `Language` field is a lazy
loader function — `entry.Language()` invokes it, compiling nothing until the first call.
`entry.HighlightQuery` is the embedded highlight query, with inheritance already resolved: a
language that declares `InheritHighlights` (TypeScript inherits JavaScript's query, for
instance) gets the composed result automatically. And because the input is Markdown with a
fenced Go block, the output includes a `{Capture: "keyword"}` range over the fenced `for` —
nested-language highlighting is wired up out of the box (more on that below).

## The output contract

```go
type HighlightRange struct {
    StartByte    uint32
    EndByte      uint32
    Capture      string
    PatternIndex int
}
```

- **`Capture`** is the raw query capture name with the `@` stripped — `"keyword"`,
  `"function.name"`, `"string"`. No mapping or normalization is applied; capture-name-to-style
  is entirely the consumer's job, same as upstream's theme layer.
- **Ranges come back sorted by `StartByte` and de-overlapped.** When two captures cover
  overlapping spans, the inner (narrower) capture wins for the overlapping bytes; adjacent
  ranges with the same capture are coalesced into one. You can walk the slice front to back
  and emit spans without an interval tree on your side.
- **`PatternIndex` is not meaningful in the output.** Overlap resolution consults pattern
  order internally (later patterns override earlier ones for identical ranges), but the field
  is zeroed in the ranges you receive — don't build logic on it.

Bytes not covered by any range are unstyled source: whitespace, plain identifiers the query
doesn't capture, and anything else the grammar's highlight query ignores. One convention to
know: the embedded queries follow nvim-treesitter's, where some capture regions as `@none`,
meaning "explicitly unstyled" — Markdown does this for `code_fence_content`, so fence bytes
between injected child captures come back as `Capture: "none"`. Map it to no style, same as
uncaptured bytes.

One thing the highlighter does *not* do: gotreesitter has no locals.scm-style scope tracking.
Upstream's `@local.definition`/`@local.scope` resolution — distinguishing a variable's
declaration from its uses — has no analogue here, and the `#is? local` predicate is a
capture-name substring check (`query_predicates.go`), not scope analysis. For scope-aware
highlighting, run your own pass over the parsed tree.

## Incremental re-highlighting

`Highlight` parses from scratch each call and releases its internal tree itself. For an editor
that re-highlights on every keystroke, use the incremental variant, which plugs into the same
edit-first contract as [incremental parsing](/docs/incremental-parsing):

1. First pass: you need a tree to be incremental *against*. `HighlightIncremental(source, nil)`
   works — a nil old tree falls back to a full parse, and you get the tree back along with the
   ranges.
2. On each edit: record it on the old tree first with `oldTree.Edit(gts.InputEdit{...})`,
   exactly as the [incremental parsing](/docs/incremental-parsing) page describes.
3. Call `hl.HighlightIncremental(newSource, oldTree)`. You get the **full** new range set for
   the whole document (not a delta) plus the new tree.
4. You own the returned tree: keep it for the next call, and `Release()` the old one when the
   returned tree is a different object.

```go
newRanges, newTree := hl.HighlightIncremental(newSrc, oldTree)
if newTree != oldTree {
    oldTree.Release()
}
oldTree, ranges = newTree, newRanges // carry forward for the next edit
```

The parse underneath reuses unchanged subtrees, so the expensive half of re-highlighting
scales with the edit, not the file. The range extraction itself still runs over the whole
tree — if you want to restyle only the changed region, intersect the returned ranges with
`gts.DiffChangedRanges(oldTree, newTree)` yourself.

For UTF-16-based callers (LSP, JavaScript interop), the whole surface has mirrors:
`HighlightUTF16([]uint16)`, `HighlightUTF16Bytes([]byte, UTF16ByteOrder)`, and
`HighlightIncrementalUTF16` / `HighlightIncrementalUTF16Bytes`, all returning
`UTF16HighlightRange` — the same contract with code-unit offsets plus `StartPoint`/`EndPoint`
in UTF-16 columns.

## Custom token sources

A few languages in the registry lex through a hand-written `TokenSource` instead of the
grammar's DFA (Go itself uses a `go/scanner` bridge). The registry exposes this as
`LangEntry.TokenSourceFactory`, a two-argument `func([]byte, *Language) TokenSource`; the
highlighter option takes a one-argument factory, so adapt with a closure — and only when the
entry actually has one:

```go
opts := []gts.HighlighterOption{}
if entry.TokenSourceFactory != nil {
    opts = append(opts, gts.WithTokenSourceFactory(func(s []byte) gts.TokenSource {
        return entry.TokenSourceFactory(s, lang)
    }))
}
hl, err := gts.NewHighlighter(lang, entry.HighlightQuery, opts...)
```

Skipping this for a language that needs it doesn't fail loudly — it parses with the DFA path
and can produce subtly different trees, so wire it whenever you build highlighters from
registry entries generically.

## Nested highlighting: code fences and injections

When you build a `Highlighter` for a language that has an injection spec registered,
`NewHighlighter` wires nested highlighting automatically — no flag to pass. The lookup keys on
`Language.Name`, and the one built-in registration is Markdown: fenced code blocks are
detected, the fence's info string (the `go` after the opening backticks) is normalized through
an alias table
(`golang` → `go`, `js` → `javascript`, `ts` → `typescript`, `py` → `python`, `sh` → `bash`,
`yml` → `yaml`, and so on), and the fence body is highlighted with the child language's own
embedded query. The child ranges land in the same output slice as the parent's, already
de-overlapped.

One honest asterisk: the Markdown fence registration lives behind `//go:build !grammar_subset`.
If you compile with the `grammar_subset` build tag (the reduced-registry build), no injection
spec is registered and Markdown highlights as plain Markdown — fences included, un-nested.

You can register your own spec for any parent language, process-wide:

```go
gts.RegisterHighlighterInjection("mytemplate", gts.HighlighterInjectionSpec{
    Query: `((directive (language) @injection.language)
             (directive_body) @injection.content)`,
    ResolveLanguage: func(hint string) (*gts.Language, string, func([]byte) gts.TokenSource, bool) {
        entry := grammars.DetectLanguageByName(hint)
        if entry == nil {
            return nil, "", nil, false
        }
        return entry.Language(), entry.HighlightQuery, nil, true
    },
})
```

The spec's query must capture the embedded region as `@injection.content`, and name the child
language either dynamically (an `@injection.language` capture whose text is the language hint)
or statically (`#set! injection.language "sql"` in the pattern). An optional
`@injection.start` capture marks where content actually begins. Registration is global — it
applies to every `Highlighter` subsequently constructed for that parent language — and a spec
whose query doesn't compile against the parent grammar surfaces as an error from
`NewHighlighter`, not a silent no-op.

This mechanism handles the highlighter's own nesting needs. If you need the actual *parse
trees* of embedded regions — not just styled ranges — that's the injection parser's job; see
[Language Injection](/docs/language-injection).

## Compile-checked example

```go title=main.go
package main

import (
	"fmt"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func main() {
	src := []byte("# Sample\n\n```go\nfor i := 0; i < 3; i++ {\n\tprintln(i)\n}\n```\n")

	entry := grammars.DetectLanguage("sample.md")
	lang := entry.Language()

	hl, err := gts.NewHighlighter(lang, entry.HighlightQuery)
	if err != nil {
		panic(err)
	}

	for _, r := range hl.Highlight(src) {
		fmt.Printf("[%3d, %3d) %-16s %q\n", r.StartByte, r.EndByte, r.Capture, src[r.StartByte:r.EndByte])
	}
}
```

The output includes Markdown's own captures for the heading and fence punctuation, and — from
the injected Go highlighter — a `keyword` range over `for`, exactly as if the fence body had
been highlighted as a standalone Go file.

## Next steps

- [Code Navigation](/docs/code-navigation) — the `Tagger`, this API's symbol-extraction
  sibling: definitions and references instead of styled ranges.
- [Language Injection](/docs/language-injection) — full parse trees for embedded languages,
  not just nested highlight ranges.
- [Incremental Parsing](/docs/incremental-parsing) — the `InputEdit` contract
  `HighlightIncremental` builds on.
- [Queries](/docs/queries) — the pattern language highlight queries are written in.
