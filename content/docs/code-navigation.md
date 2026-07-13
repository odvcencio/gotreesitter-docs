---
title: Code Navigation
description: Extract symbol definitions and references with the Tagger — tags queries, the definition/reference convention, and inferred queries for all 206 grammars.
nav_group: Using the Parser
order: 6
---

Code navigation is the "jump to definition" / "find all references" layer: instead of styled
ranges, you want a flat list of symbols — this function is *defined* here, *called* there —
with precise source locations for both the whole construct and the identifier inside it. In
gotreesitter that layer is the `Tagger`, the symbol-extraction sibling of the
[syntax highlighter](/docs/syntax-highlighting): same construction shape, same query engine
underneath, different output.

The model follows upstream tree-sitter's
[code navigation](https://tree-sitter.github.io/tree-sitter/4-code-navigation.html) system:
a *tags query* whose capture names follow a fixed convention identifies definition and
reference sites, and the runtime turns matches into tag records. Upstream ships this as the
`tree-sitter tags` CLI over per-grammar `tags.scm` files; gotreesitter ships it as a library
API, with a twist on where the queries come from (below).

If the query language is new to you, start with [Queries](/docs/queries) — a tags query is an
ordinary query with a naming convention, nothing more.

## Quick start

```go
entry := grammars.DetectLanguage("main.go")
lang := entry.Language()

tagger, err := gts.NewTagger(lang, grammars.ResolveTagsQuery(*entry))
if err != nil {
    log.Fatal(err)
}

tags := tagger.Tag([]byte("package main\n\nfunc Add(a, b int) int { return a + b }\nfunc main() { Add(1, 2) }\n"))
for _, t := range tags {
    fmt.Printf("%-20s %-6s bytes [%d, %d)\n", t.Kind, t.Name, t.NameRange.StartByte, t.NameRange.EndByte)
}
```

On that input the tagger reports `definition.function` for `Add` and `main`, and
`reference.call` for the `Add(1, 2)` call site — three tags, each carrying both the span of
the whole construct and the span of just the identifier.

## Where tags queries come from

Here is the part that differs from both upstream and this project's own highlighter, and it's
worth being precise about: **none of the 206 built-in grammars ships a tags query.** Every
`LangEntry.TagsQuery` field is empty. Reading it directly gets you an empty string and a
query-compile error from `NewTagger`.

The supported path is `grammars.ResolveTagsQuery(entry)`:

- If the entry has an explicit `TagsQuery` (yours might, via your own `grammars.Register`
  entry), that's returned as-is.
- Otherwise the query is **inferred**: a catalog of around 40 candidate patterns — the common
  definition and call shapes across mainstream grammars (`function_declaration`,
  `method_definition`, `class_declaration`, `call_expression`, `method_invocation`, and so
  on) — is filtered against the grammar's actual node types, and the patterns whose node
  names all exist in the grammar are concatenated into the query. The result is cached per
  language name.
- Go gets a hand-tuned override instead of inference, with field-constrained patterns
  (`name: (identifier)`) that are more precise than the generic shapes.

The honest asterisk that follows from inference: coverage varies by grammar. A language whose
grammar uses the common node-type vocabulary gets a useful query for free; a grammar with
idiosyncratic node names may match few or none of the candidate patterns, and
`ResolveTagsQuery` returns whatever subset applies — possibly a very short query. For
production navigation over a specific language, write a real tags query for it (upstream's
per-grammar `tags.scm` files are a good starting point) and pass it to `NewTagger` yourself;
inference is the batteries-included default, not the ceiling. Note also that inference may
trigger grammar loading for the language, since it needs the node-type list.

## The capture convention

A tags query names things with a two-capture pattern:

```scheme
(function_declaration name: (identifier) @name) @definition.function
(call_expression function: (identifier) @name) @reference.call
```

- The **enclosing node** is captured as `@definition.X` or `@reference.Y` — the capture name
  becomes the tag's `Kind`, verbatim.
- The **identifier** is captured as `@name` — its text becomes the tag's `Name`, its span the
  tag's `NameRange`.

Two fallback rules: a match with no `@definition.*`/`@reference.*` capture is dropped
entirely, and a match with a kind capture but no `@name` still produces a tag whose `Name`
falls back to the full text of the captured node's range.

The kind vocabulary is open — `definition.function`, `definition.method`, `definition.class`,
`definition.type`, `definition.constant`, `reference.call`, `reference.implementation`,
whatever your query emits. gotreesitter doesn't validate kinds against a fixed list.

## The output

```go
type Tag struct {
    Kind      string // "definition.function", "reference.call", ...
    Name      string // the captured symbol text
    Range     Range  // full span of the tagged node
    NameRange Range  // span of the @name capture
}
```

`Range` bundles byte offsets and `Point` (row/column) pairs, so a tag is directly usable for
both index storage and editor UI. Definition-versus-reference is the `Kind` prefix — there is
no separate boolean field, so filter with `strings.HasPrefix(t.Kind, "definition.")` or match
the full kind strings you care about. There is no docs-comment field either (upstream's tags
CLI extracts adjacent doc comments; the library `Tagger` here does not) — if you need doc
text, walk from the tagged node to its preceding sibling comment via the
[Node API](/docs/syntax-trees-and-nodes).

## Three ways to run it

- **`Tag(source []byte) []Tag`** — parse and tag in one call. The everyday entry point.
- **`TagTree(tree *Tree) []Tag`** — tag a tree you already have. If you're highlighting,
  linting, and tagging the same file, parse once and share the tree instead of paying three
  parses.
- **`TagIncremental(source, oldTree) ([]Tag, *Tree)`** — the editor path. Same edit-first
  contract as everything else in the engine: call `oldTree.Edit(...)` with the
  [`InputEdit`](/docs/incremental-parsing), then `TagIncremental` returns the full new tag
  set plus the new tree, which you own (release the old one when it's a different object).

All three have UTF-16 mirrors for LSP-shaped callers — `TagUTF16`, `TagUTF16Bytes`,
`TagTreeUTF16`, `TagIncrementalUTF16`, `TagIncrementalUTF16Bytes` — returning `UTF16Tag`
with code-unit coordinates.

Like the highlighter, languages with a custom lexer bridge need their token source wired
through: the option is `WithTaggerTokenSourceFactory`, and the registry's two-argument
`LangEntry.TokenSourceFactory` adapts with the same closure pattern shown on the
[syntax highlighting](/docs/syntax-highlighting) page.

## The lightweight alternative

For "give me the definitions in this file" without any query at all, the engine also has a
pair of one-pass helpers: `gts.ExtractDefinitionSpans(tree) []DefinitionSpan` and
`gts.ExtractCalls(tree) []CallRef`. They walk the tree once with hard-coded knowledge of
common declaration and call shapes, returning compact language-neutral records
(`Lang`, `Kind`, `Name`, byte spans for node and name). Deliberately conservative — unknown
languages and shapes are skipped, not guessed — and not query-configurable; reach for them in
batch-indexing tools where you want the cheap 90%, and for the `Tagger` when you want control
over what counts as a symbol.

## Compile-checked example

```go title=main.go
package main

import (
	"fmt"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func main() {
	src := []byte("package main\n\nfunc Add(a, b int) int { return a + b }\nfunc main() { Add(1, 2) }\n")

	entry := grammars.DetectLanguage("main.go")
	lang := entry.Language()

	tagger, err := gts.NewTagger(lang, grammars.ResolveTagsQuery(*entry))
	if err != nil {
		panic(err)
	}

	for _, t := range tagger.Tag(src) {
		role := "ref"
		if strings.HasPrefix(t.Kind, "definition.") {
			role = "def"
		}
		fmt.Printf("%s %-20s %-6s line %d\n", role, t.Kind, t.Name, t.NameRange.StartPoint.Row+1)
	}
}
```

```text
def definition.function  Add    line 3
def definition.function  main   line 4
ref reference.call       Add    line 4
```

A symbol index for a whole repository is this loop plus file walking: detect the language per
file, resolve (or supply) its tags query, tag, and store `(Kind, Name, path, NameRange)`
tuples. Definitions become your jump-to-definition table; references become find-all-references.

## Next steps

- [Syntax Highlighting](/docs/syntax-highlighting) — the same construction pattern producing
  styled ranges instead of symbols; the two share trees via `TagTree`.
- [Queries](/docs/queries) — the pattern language for writing a better tags query than the
  inferred one.
- [Incremental Parsing](/docs/incremental-parsing) — the edit contract behind
  `TagIncremental`.
- [Languages](/docs/languages) — `LangEntry`, `DetectLanguage`, and registering your own
  entry with an explicit `TagsQuery`.
