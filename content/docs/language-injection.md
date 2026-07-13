---
title: Language Injection
description: Parse multi-language documents — Markdown with fenced code, HTML with script tags, templates with embedded expressions — into a parent tree plus real child trees.
nav_group: Using the Parser
order: 7
---

Real documents mix languages: Markdown carries fenced Go, HTML carries JavaScript and CSS, a
template language carries SQL. Tree-sitter's answer is *injection* — parse the document with
the parent grammar, find the embedded regions with a query, then parse each region with its
own grammar. Upstream covers the underlying mechanics under
[multi-language documents](https://tree-sitter.github.io/tree-sitter/using-parsers/3-advanced-parsing.html),
where the primitive is manually setting included ranges on a parser; gotreesitter ships that
primitive too, plus an `InjectionParser` that automates the whole detect-and-parse loop,
recursively, with incremental reparse support.

If you only want embedded regions *styled* — fenced code blocks with syntax colors — you
don't need any of this: the [highlighter](/docs/syntax-highlighting) wires nested
highlighting automatically. Reach for the injection parser when you need the actual child
**trees**: linting the Go inside a Markdown fence, extracting symbols from `<script>` blocks,
running [queries](/docs/queries) against embedded SQL.

## The InjectionParser

```go
ip := gts.NewInjectionParser()

ip.RegisterLanguage("markdown", grammars.MarkdownLanguage())
ip.RegisterLanguage("go", grammars.GoLanguage())

err := ip.RegisterInjectionQuery("markdown", `
(fenced_code_block
  (info_string (language) @injection.language)
  (code_fence_content) @injection.content)
`)

result, err := ip.Parse(src, "markdown")
```

Setup is three steps: register every language that can appear (parent or child) under a name,
register an injection query per parent language, and parse. `RegisterInjectionQuery` compiles
the query against the registered parent grammar immediately — a typo'd node name fails at
registration, not at parse time — and requires the parent to have been registered first.

`Parse(source, parentLang)` parses the document with the parent grammar, runs the parent's
injection query, parses each detected region with its child grammar, and then **recurses**: if
a child language also has an injection query registered, its embedded regions are found and
parsed too (Markdown inside a doc-comment inside Go inside a Markdown fence, if you insist).
Recursion is bounded by `SetMaxDepth` — the default is 10 levels; values ≤ 0 restore the
default.

Two deliberately quiet behaviors: a parent language with **no injection query registered**
parses fine and returns empty `Injections` (no error — the document just has no known
embeddings), and a detected region whose language is **not registered** still produces an
`Injection` entry, with `Tree == nil` — you see that the region exists and what language it
claims to be, you just don't get a parse of it.

## The injection query

The query runs against the parent tree and uses two capture conventions, the same ones
upstream and the [highlighter](/docs/syntax-highlighting) use:

- **`@injection.content`** — required. The node whose span is the embedded region.
- The child **language name** comes from one of two places, checked in this order:
  1. Static: a `#set! injection.language "x"` directive on the pattern.
  2. Dynamic: an `@injection.language` capture, whose node *text* is the language name.

A fixed-language template field is the static form:

```scheme
((sql_block (content) @injection.content)
  (#set! injection.language "sql"))
```

Markdown fences are the dynamic form — the language is written in the document itself:

```scheme
(fenced_code_block
  (info_string (language) @injection.language)
  (code_fence_content) @injection.content)
```

The language name is matched against `RegisterLanguage` names verbatim. If your documents open
fences with `golang` but you registered `"go"`, either register the language under both names or
normalize before registering — the injection parser does not apply alias tables (the
highlighter's fence handling does, but that's its own registration, not this API).

## The result

```go
type InjectionResult struct {
    Tree       *Tree       // the parent language's parse tree
    Injections []Injection // child regions, ordered by position
}

type Injection struct {
    Language string  // detected language name, e.g. "go"
    Tree     *Tree   // the region's parse tree, or nil if unregistered
    Ranges   []Range // the source ranges this tree covers
    Node     *Node   // the parent-tree node that triggered the injection
}
```

The property that makes results directly usable: **child trees are document-relative.** A
child tree's root does not start at byte 0 — it starts at the injection's offset in the
document, so every node's `StartByte`/`EndByte` and `Point` coordinates index into the *whole
document's* source. `childRoot.Text(documentSource)` just works; positions from a child tree
can be handed to the same editor buffer as positions from the parent tree, no offset
arithmetic on your side.

Under the hood this is also a performance design, not just a convenience (from the engine
source): a single-range injection is parsed as just its byte slice on an incremental-class
arena — a 16 KB slab instead of the 2 MB full-parse arena — and the finished tree's
coordinates are then rebased into document space. A README with fifty small fences costs
fifty small parses, not fifty full-document parses. Multi-range injections take the other
path: the child parser parses the full source with `SetIncludedRanges` restricting it to the
region (see below).

## Lifetimes and concurrency

Two contracts to respect, both stated plainly:

- **An `InjectionParser` is not safe for concurrent use.** It caches per-language parsers and
  mutates shared state during parses. One goroutine per `InjectionParser`, or a pool.
- **Each `Parse` call releases the previous result's trees.** The parser holds its last
  result so arenas can be reused; the trees you got from call N are invalid after call N+1.
  Copy out what you need (tags, ranges, text) before reparsing, or use `tree.Copy()` if you
  genuinely need a surviving tree.

## Incremental reparsing

`ParseIncremental(source, parentLang, oldResult)` is the editor path, and it composes with
the standard [edit-first contract](/docs/incremental-parsing): apply your `InputEdit` to
`oldResult.Tree` first, then call `ParseIncremental` with the new source. The parent parse is
incremental in the usual subtree-reusing way, and the child trees get their own reuse pass —
a child tree is carried over untouched when the same language and ranges are detected again
and no changed range overlaps it (computed via `DiffChangedRanges` on the parent trees). Edit
one fence in a fifty-fence README and the other forty-nine child trees are reused, not
reparsed.

The full API surface mirrors the rest of the engine for UTF-16 callers: `ParseUTF16`,
`ParseUTF16Bytes`, `ParseIncrementalUTF16`, and `ParseIncrementalUTF16Bytes` return
`UTF16InjectionResult` with ranges in code units.

## The lower-level primitive: included ranges

The injection parser is a loop over something you can also drive directly:
`Parser.SetIncludedRanges([]Range)` restricts a parse to a set of byte ranges — the parser
lexes and parses *only* those spans, and tokens outside them are filtered internally. This is
the exact mechanism upstream describes for multi-language documents, and it's how gotreesitter
parses multi-range injections (an embedded region interrupted by template syntax, say).

```go
parser := gts.NewParser(grammars.GoLanguage())
parser.SetIncludedRanges([]gts.Range{{StartByte: fenceStart, EndByte: fenceEnd /* plus Points */}})
tree, err := parser.Parse(fullDocument) // parses only the fence bytes
```

`SetIncludedRanges` normalizes its input — empty ranges are dropped, ranges are sorted, and
overlapping or adjacent ranges are merged — and `IncludedRanges()` reads the normalized set
back. Passing an empty or nil slice restores whole-document parsing. UTF-16 variants exist
here too: `SetIncludedUTF16Ranges(source, ranges)` and
`SetIncludedUTF16ByteRanges(source, order, ranges)`.

Use the raw primitive when you already know the regions (your template engine told you) and
don't need detection, recursion, or result management; use `InjectionParser` when the regions
live in the document and you'd otherwise be reimplementing its loop.

## Compile-checked example

```go title=main.go
package main

import (
	"fmt"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func main() {
	src := []byte("# Sample\n\n```go\nfunc Add(a, b int) int { return a + b }\n```\n")

	ip := gts.NewInjectionParser()
	ip.RegisterLanguage("markdown", grammars.MarkdownLanguage())
	ip.RegisterLanguage("go", grammars.GoLanguage())

	err := ip.RegisterInjectionQuery("markdown", `
(fenced_code_block
  (info_string (language) @injection.language)
  (code_fence_content) @injection.content)
`)
	if err != nil {
		panic(err)
	}

	result, err := ip.Parse(src, "markdown")
	if err != nil {
		panic(err)
	}

	fmt.Println("parent root:", result.Tree.RootNode().Type(result.Tree.Language()))
	for _, inj := range result.Injections {
		if inj.Tree == nil {
			fmt.Printf("unregistered language %q at %v\n", inj.Language, inj.Ranges)
			continue
		}
		root := inj.Tree.RootNode()
		fmt.Printf("%s injection, bytes [%d, %d): %s\n",
			inj.Language, root.StartByte(), root.EndByte(),
			root.Type(grammars.GoLanguage()))
	}
}
```

The Go injection's root node reports document-relative byte offsets — it starts where the
fence content starts in the Markdown source, not at 0 — and its subtree is a normal Go parse
you can hand to a [query](/docs/queries), the [tagger](/docs/code-navigation), or any other
tree consumer.

## Next steps

- [Syntax Highlighting](/docs/syntax-highlighting) — the automatic nested-highlighting layer
  built on the same query conventions, for when styled ranges are all you need.
- [Incremental Parsing](/docs/incremental-parsing) — the `InputEdit` contract behind
  `ParseIncremental`.
- [Queries](/docs/queries) — the pattern language injection queries are written in.
- [Languages](/docs/languages) — `LangEntry` and the per-language accessors
  (`grammars.MarkdownLanguage()` and friends) used to register languages here.
