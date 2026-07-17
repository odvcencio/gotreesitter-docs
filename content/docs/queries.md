---
title: Queries
description: Find syntax patterns in a parsed tree with tree-sitter's S-expression query language and gotreesitter's query engine.
nav_group: Using the Parser
order: 3
---

A query is a set of S-expression patterns matched against a parsed tree. Queries answer questions like "every exported function," "every string literal inside a loop," or "every import statement" without a hand-written tree walk. The same engine backs gotreesitter's [syntax highlighter](/docs/syntax-highlighting), [symbol tagger](/docs/code-navigation), and [multi-language injection parser](/docs/language-injection) — all three compile a query and run it against your tree; this page covers the query language and the `Query`/`QueryCursor` API directly.

This page assumes you already have a `*gotreesitter.Tree` — see [Syntax Trees and Nodes](/docs/syntax-trees-and-nodes) if you need that first.

## Quick start

```go
lang := grammars.GoLanguage()

parser := gts.NewParser(lang)
tree, err := parser.Parse([]byte(src))
if err != nil {
    log.Fatal(err)
}
defer tree.Release()

q, err := gts.NewQuery(`
(function_declaration
  name: (identifier) @func.name) @func.def
`, lang)
if err != nil {
    log.Fatal(err)
}

cursor := q.Exec(tree.RootNode(), lang, tree.Source())
for {
    match, ok := cursor.NextMatch()
    if !ok {
        break
    }
    for _, cap := range match.Captures {
        fmt.Printf("%s: %s\n", cap.Name, cap.Text(tree.Source()))
    }
}
```

> [!TIP] Compile-time safety
> `gts.NewQuery(source string, lang *gts.Language) (*gts.Query, error)` compiles a query against a specific language, resolving every node type and field name up front — a query written for the wrong grammar fails at compile time (`query: unknown node type "foo"`), not silently at match time. A compiled `*Query` is safe to reuse and to share across goroutines.

## The pattern language

A pattern is a parenthesized S-expression rooted at a node type:

```scheme
(function_declaration
  name: (identifier) @name
  parameters: (parameter_list) @params
  body: (block) @body) @func
```

**Captures** — `@name` — label a node (or a whole subtree) so you can read it back from `match.Captures`. A pattern can carry any number of captures at any depth, including one on the outermost node (`@func` above).

**Field constraints** — `field: (pattern)` — require the child to sit in a specific grammar field, not just anywhere among the node's children. This is how the example above pins `@name` to the function's name rather than, say, a parameter that happens to also be an identifier.

**Negated fields** — `!field` — require a field to be *absent*. `(function_declaration !result name: (identifier) @name)` matches only functions with no declared return value.

**Wildcards** — `_` matches any node at all, named or anonymous (keywords, punctuation included). `(_)` — parenthesized — matches any *named* node only. This distinction is real and load-bearing:

```go
// (function_declaration _ @child)   -> captures "func", the name, the
//                                       parameter list, the result type,
//                                       and the body: every child.
// (function_declaration (_) @child) -> captures every child except "func":
//                                       anonymous keyword/punctuation tokens
//                                       are excluded.
```

**Anchors** — `.` — require adjacency with no intervening (named) sibling. `.` before the first child pattern anchors it to the parent's first named child; `.` after the last one anchors it to the last; `.` between two sibling patterns requires them to be immediately adjacent, with the matcher backtracking across earlier siblings to find a pair that satisfies it.

```go
// On "func F(a int, b string, c bool) {}":
q, _ := gts.NewQuery(`(parameter_list . (parameter_declaration) @first)`, lang)
// -> 1 match, @first = "a int". The leading `.` pins the capture to the
//    first named child; without it, all three parameters match.

// `.` counts *named* siblings only, so the anonymous "," between parameters
// doesn't break adjacency — every consecutive pair matches:
q, _ = gts.NewQuery(`(parameter_list
  (parameter_declaration) @a . (parameter_declaration) @b)`, lang)
// -> 2 matches: ("a int", "b string") and ("b string", "c bool").
```

**Quantifiers** — `?`, `*`, `+` — suffix a pattern to make it optional, zero-or-more, or one-or-more. `?` is straightforward: the capture is simply missing from the match when the optional node isn't there. `*`/`+` on a *child* pattern are more specific than they look: they collect one **contiguous run** of matching siblings into a single match, stopping at the first sibling that doesn't match — which includes punctuation. Two adjacent comment nodes are a contiguous run and both land in one match:

```go
// source: two "//" comments directly followed by a func decl
q, _ := gts.NewQuery(`(source_file (comment)+ @doc)`, lang)
// -> 1 match, @doc captured twice (both comments)
```

but a comma-separated parameter list is *not* contiguous — the "," tokens between `parameter_declaration` nodes break the run:

```go
q, _ := gts.NewQuery(`(parameter_list (parameter_declaration)+ @param)`, lang)
// on "F(a int, b string, c bool)": 1 match, @param captured once ("a int")
```

For "capture every item in a comma/punctuation-separated list," skip the quantifier entirely and let the pattern match repeatedly on its own — an **unquantified** repeated child pattern produces one match per occurrence, which is the idiom highlight queries actually use:

```go
q, _ := gts.NewQuery(`(parameter_list (parameter_declaration) @param)`, lang)
// on the same source: 3 separate matches, one @param capture each
```

**Alternation** — `[...]` — matches any one of several node types (or string literals) at a position:

```scheme
[
  (function_declaration)
  (method_declaration)
] @decl
```

**String literals** — `"text"` — match an anonymous token by its exact text, e.g. `"return"` or `"+"`. Useful inside an alternation for keyword sets: `["if" "else"] @keyword`.

**Grouping** — `(pattern1 pattern2 ...)` without a leading node type groups a run of sibling patterns, mainly so a quantifier can apply to the whole group at once (`((line_comment) (line_comment))* @doc-block`).

**The `ERROR` node** — `(ERROR) @err` matches the parser's error nodes: spans the parser couldn't assign to any grammar rule (the same nodes `node.IsError()` reports — see [Syntax Trees and Nodes](/docs/syntax-trees-and-nodes)). `ERROR` resolves to the real error symbol at compile time, so patterns like `(ERROR (identifier) @salvaged)` work for picking recognizable pieces out of broken regions.

**The `MISSING` node — not supported.** Upstream's query language can match zero-width missing nodes with `(MISSING)`, `(MISSING identifier)`, or `(MISSING ";")`. gotreesitter's compiler currently accepts the `MISSING` keyword but compiles it to a plain any-node wildcard — the matcher never checks missing-ness, so `(MISSING)` silently matches everything and the child form matches the wrong shape entirely. Don't use it; find missing nodes by walking the tree and checking `node.IsMissing()` instead.

**Supertypes — not supported in patterns.** Upstream grammars declare supertype rules (like `expression`), and upstream queries can write `(expression)` to match any of its subtypes, or qualify one as `expression/identifier`. gotreesitter does not expand supertypes at pattern positions: `(expression)` matches only a node literally named `expression`, and a `parent/child` name silently resolves to just the rightmost segment — `expression/identifier` behaves exactly like `(identifier)`. The supertype tables do exist on the `Language` (they power ancestor-type predicates), but pattern-position matching doesn't consult them. Spell out the subtypes you want in an alternation `[...]` instead.

The pattern language is upstream tree-sitter's, and upstream's query docs are the canonical spec for it: [syntax](https://tree-sitter.github.io/tree-sitter/using-parsers/queries/1-syntax.html), [operators](https://tree-sitter.github.io/tree-sitter/using-parsers/queries/2-operators.html), and [predicates and directives](https://tree-sitter.github.io/tree-sitter/using-parsers/queries/3-predicates-and-directives.html). Where gotreesitter's engine differs — the two unsupported cases above, and predicate evaluation below — this page says so explicitly.

## Predicates

A predicate is a parenthesized directive following the patterns it applies to, written `(#name? args...)`. Unlike the reference C tree-sitter library — which only parses predicates and leaves evaluation to whatever binding consumes the query — gotreesitter's engine evaluates predicates itself and filters matches before they reach you.

| Predicate | Meaning |
|---|---|
| `#eq?` / `#not-eq?` | Capture text equals (or doesn't) a literal or another capture's text |
| `#match?` / `#not-match?` | Capture text matches (or doesn't) a Go regexp |
| `#any-of?` / `#not-any-of?` | Capture text is (or isn't) one of a list of literals |
| `#any-eq?` / `#any-not-eq?` | At least one node under a repeated capture equals (or doesn't) a value |
| `#any-match?` / `#any-not-match?` | At least one node under a repeated capture matches (or doesn't) a regexp |
| `#lua-match?` | Capture text matches a Lua string pattern (common in ported Neovim queries) |
| `#has-ancestor?` / `#not-has-ancestor?` | Capture has (or lacks) an ancestor of a given type |
| `#has-parent?` / `#not-has-parent?` | Capture's immediate parent is (or isn't) a given type |
| `#is?` / `#is-not?` | Capture has (or lacks) a named property; checks a simple property/capture-name relationship (e.g. `local`, `function`), not full scope analysis |
| `#count?` | Number of nodes under a capture satisfies a comparison (`>`, `<`, `>=`, `<=`, `==`, `!=`) against an integer |
| `#is-exported?` | Capture text starts with an uppercase letter (a Go-flavored convenience) |

```go
q, _ := gts.NewQuery(`
(function_declaration
  name: (identifier) @name
  (#match? @name "^[A-Z]"))
`, lang)
```

Two directives are also recognized but not enforced by the matcher itself: `#set!` attaches arbitrary key/value metadata to a pattern, read back with `QueryMatch.SetValues(q, key)` (this is how gotreesitter's injection parser reads an `injection.language` directive out of a query match), and `#offset!` is parsed and stored but not applied automatically. `#select-adjacent!` and `#strip!` *are* applied: the former filters a capture list down to nodes byte-adjacent to an anchor capture, the latter rewrites a capture's returned text by stripping a regexp match (via `QueryCapture.TextOverride`, surfaced through `cap.Text(source)`).

## Running a query

`Query.Execute(tree *gts.Tree) []QueryMatch` runs against a tree's root and materializes every match. `Query.ExecuteNode(node, lang, source)` starts from an arbitrary node instead of the tree root — useful for querying a single function body. `Query.ExecuteInto(tree, dst)` appends into a caller-owned slice to avoid a fresh allocation on repeated calls.

For large files or early termination, use the streaming cursor instead of materializing everything:

```go
cursor := q.Exec(tree.RootNode(), lang, tree.Source())
cursor.SetByteRange(0, 4096)   // restrict matches to a byte window
cursor.SetMatchLimit(100)      // cap match count; DidExceedMatchLimit() reports overflow

for {
    match, ok := cursor.NextMatch()
    if !ok {
        break
    }
    _ = match
}
```

`QueryCursor` also has `SetPointRange` (row/column window), `SetUTF16Range` (the UTF-16 equivalent, for trees produced by a UTF-16 parse), `SetMaxStartDepth`, and `NextCapture()` — a capture-at-a-time alternative to `NextMatch()` that drains the same match stream one capture at a time. A `QueryCursor` is single-use and **not** safe for concurrent use; get one per goroutine from `q.Exec(...)` (the `*Query` itself is fine to share).

Each `QueryMatch` carries `PatternIndex` (which top-level pattern in the source matched — useful with multi-pattern queries and `q.PredicatesForPattern(idx)`) and `Captures []QueryCapture`, where each `QueryCapture` has `Name`, `Node *gts.Node`, and a `Text(source []byte) string` method that respects any `#strip!` override.

If you keep queries in `.scm` files, `cmd/tsquery` generates a typed Go wrapper from one (`tsquery -input FILE.scm -lang LANG -output FILE.go -package PKG`) — it is a code generator for embedding a query as compiled Go source, not a query runner.

## Compile-checked example

The following compiles and runs against `github.com/odvcencio/gotreesitter/grammars`:

```go title=main.go
package main

import (
	"fmt"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const src = `package main

func Add(a, b int) int {
	return a + b
}

func subtract(a, b int) int {
	return a - b
}
`

func main() {
	lang := grammars.GoLanguage()

	parser := gts.NewParser(lang)
	tree, err := parser.Parse([]byte(src))
	if err != nil {
		panic(err)
	}
	defer tree.Release()

	q, err := gts.NewQuery(`
(function_declaration
  name: (identifier) @func.name) @func.def
`, lang)
	if err != nil {
		panic(err)
	}

	cursor := q.Exec(tree.RootNode(), lang, tree.Source())
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, cap := range match.Captures {
			fmt.Printf("%s: %s\n", cap.Name, cap.Text(tree.Source()))
		}
	}

	// Exported-only functions, via a predicate.
	exported, err := gts.NewQuery(`
(function_declaration
  name: (identifier) @func.name
  (#match? @func.name "^[A-Z]"))
`, lang)
	if err != nil {
		panic(err)
	}
	fmt.Println("exported funcs:", len(exported.Execute(tree)))
}
```

This prints both function definitions and their names, then reports `exported funcs: 1` (only `Add` starts with an uppercase letter).

Queries are grammar-specific by construction, but the pattern language itself is the same across all 206 grammars. Once you have a tree in hand, see [incremental parsing](/docs/incremental-parsing) for how to keep it up to date across edits instead of reparsing from scratch.
