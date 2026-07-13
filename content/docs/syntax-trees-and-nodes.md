---
title: Syntax Trees and Nodes
description: The Tree and Node types in depth — lifecycle, positions, navigation, fields, and text extraction.
nav_group: Using the Parser
order: 1
---

Every parse in gotreesitter returns a `*gotreesitter.Tree`. The tree owns a `*Node` root, the
source bytes it was parsed from, and a reference to the `*Language` that defines what its node
types mean. Almost everything you do with a parse result — reading a node's type, walking to its
children, slicing out its source text — goes through the `Node` API described here.

```go
lang := grammars.GoLanguage()
parser := gts.NewParser(lang)

tree, err := parser.Parse(src)
if err != nil {
    panic(err)
}
defer tree.Release()

root := tree.RootNode()
fmt.Println(root.Type(lang)) // "source_file"
```

## The tree

A `*Tree` is a small handle around three things you'll use constantly:

- `tree.RootNode() *Node` — the top of the syntax tree.
- `tree.Source() []byte` — the exact bytes that were parsed. Node positions are offsets into
  this slice.
- `tree.Language() *Language` — the grammar the tree was parsed with. Most `Node` methods that
  turn a numeric symbol into a readable name need this.

Trees are backed by arena-allocated nodes, not individual heap objects, which is why they come
with explicit lifecycle methods instead of relying purely on the garbage collector:

- `tree.Copy() *Tree` returns an independent tree with its own node objects. Source bytes and the
  language pointer are shared read-only, but edits to one tree (see incremental parsing) never
  mutate the other's spans. Use this when you need to hand a tree to code that might edit it while
  you keep working with the original.
- `tree.Release()` returns the tree's arena memory to the allocator. After `Release`, the tree and
  every node reachable from it are invalid — don't call methods on them, and don't hold onto
  `*Node` pointers from a released tree. `defer tree.Release()` right after a successful parse is
  the normal pattern.

`Tree` also has an `Edit` method and a `ChangedRanges` method for incremental reparsing; that
workflow gets its own treatment on the [incremental parsing](/docs/incremental-parsing) page. This
page focuses on reading a tree you already have.

## The node model

A `Node` is a span of source text tagged with a grammar symbol. Two methods answer "what is this":

- `node.Symbol() Symbol` — the raw numeric ID of the node's grammar rule or token. Symbols are
  cheap to compare (`==`) and are what the parser itself works with internally.
- `node.Type(lang) string` — the human-readable name for that symbol, e.g. `"function_declaration"`
  or `"identifier"`. This is what you'll print and match against in almost all application code.

Going from a name to a comparable symbol once, instead of calling `Type` in a hot loop, is
supported directly:

```go
identifierSym, _ := lang.SymbolByName("identifier")
if node.Symbol() == identifierSym {
    // cheaper than node.Type(lang) == "identifier" on every node
}
```

### Named vs. anonymous

`node.IsNamed() bool` distinguishes the two kinds of node that make up a tree, matching
tree-sitter's convention:

- **Named** nodes correspond to a rule in the grammar — `function_declaration`, `identifier`,
  `binary_expression`. These are what `SExpr` prints and what `NamedChild` walks.
- **Anonymous** nodes are literal tokens the grammar spells out directly — keywords like `func`,
  punctuation like `(` and `,`. They're still real nodes with real positions; they just don't carry
  grammar-meaningful structure of their own.

### Error and recovery flags

Four boolean methods describe how a node relates to parse errors:

- `node.IsError() bool` — true only for an explicit `ERROR` node: a span the parser couldn't
  assign to any grammar rule. `node.Type(lang)` for such a node returns `"ERROR"`.
- `node.IsMissing() bool` — true for a zero-width node error recovery inserted because the grammar
  required a token that wasn't in the input (a missing `)`, for instance).
- `node.IsExtra() bool` — true for nodes the grammar marked `extra` (typically comments and
  whitespace) that sit outside the core parse structure.
- `node.HasError() bool` — true for a node **or any of its descendants** containing a parse error.
  This is the one to check on a root node to answer "did this file parse cleanly?" — `IsError` only
  tells you about one specific node, not the subtree under it.

```go
if root.HasError() {
    // something under root failed to parse; walk to find the ERROR/MISSING nodes
}
```

## Positions: bytes and points

Every node carries its position two ways, and both matter for different jobs:

- `node.StartByte() / node.EndByte() uint32` — an exclusive byte range into `tree.Source()`. Fast,
  unambiguous, and what you want for slicing source text or storing spans in an index.
- `node.StartPoint() / node.EndPoint() Point` — a `Point{Row, Column uint32}` pair, both zero-based.
  This is what you want for editor UIs and diagnostics that talk in terms of lines and columns.
- `node.Range() Range` bundles all four into one `Range{StartByte, EndByte, StartPoint, EndPoint}`
  value, handy when a function wants "the span" as a single argument.

One detail matters and is easy to get wrong: **`Point.Column` counts bytes since the start of the
row, not Unicode characters.** This matches the C tree-sitter runtime. A non-ASCII character before
your node shifts the column by its UTF-8 byte width, not by one:

```go
src := []byte("package main\n\nvar x = \"café\" + y\n")
// "café" is 4 runes but 5 bytes (é is 2 bytes in UTF-8).
// The `y` identifier that follows sits at StartPoint{Row: 2, Column: 18} —
// 18 is its byte offset from the start of the row, not its rune offset.
```

If you need rune-aware columns for a UTF-16-based editor protocol (e.g. LSP), parse with
`ParseUTF16` instead and use the tree's `UTF16RangeForNode` / `UTF16OffsetForByte` conversions
rather than reinterpreting `Point.Column` yourself.

## Navigating the tree

Given any node, you can move to its relatives without re-walking from the root:

```go
fn := root.NamedChild(1) // the function_declaration

fn.Parent()          // == root
fn.ChildCount()      // all children, named and anonymous
fn.Child(1)          // the i-th child, any kind
fn.NamedChildCount() // only named children
fn.NamedChild(0)     // the i-th named child
fn.NextSibling()     // nil if fn is the last child
fn.PrevSibling()     // nil if fn is the first child
fn.Children()        // []*Node of every child, named and anonymous
```

`Child`/`NamedChild` are the two you'll reach for most: `Child(i)` indexes every child including
punctuation and keywords, while `NamedChild(i)` skips straight to the grammar-meaningful ones. For
the function above, `fn.Children()` yields `func`, `identifier`, `parameter_list`, `type_identifier`,
`block` — five children, but only four of them (`identifier` on) are named.

Walking every node in a subtree by hand-rolling `Child`/`NamedChild` recursion works fine for small
trees or one-off lookups. For traversing large trees repeatedly, or when you need to move
backwards as well as forwards, see [Tree Cursors](/docs/tree-cursors) — `TreeCursor` and the
package-level `Walk` helper are built for that.

## Fields

Grammars name some children with **fields** — `body`, `name`, `parameters` — so you can find a
specific child without knowing its positional index or worrying about grammar changes shifting it.
Field lookups take the `*Language`, the same as `Type`:

```go
body := fn.ChildByFieldName("body", lang)  // the block node
fmt.Println(body.Type(lang))               // "block"

for i := 0; i < fn.ChildCount(); i++ {
    if name := fn.FieldNameForChild(i, lang); name != "" {
        fmt.Println(i, name) // 1 name, 2 parameters, 3 result, 4 body
    }
}
```

`ChildByFieldName` returns the first child with that field (most fields are single-valued;
grammars with repeatable fields, like multiple `case:` arms, still only give you the first match
this way — walk `Children()` and check `FieldNameForChild` for each one if you need all of them).
`FieldNameForChild` is the inverse: given a child index, what field (if any) does the grammar
assign it. Both return `""`/`nil` cleanly when there's no such field, so you don't need to guard
every call with a fields-exist check.

## Text and debugging

Two methods turn a node back into something readable:

- `node.Text(source []byte) string` — the exact source bytes the node spans. Pass `tree.Source()`
  (or your own copy of the same bytes) — this is a plain slice-and-convert, so it's cheap, but it
  does allocate a new string each call.
- `node.SExpr(lang) string` — a tree-sitter-style S-expression, e.g.
  `(function_declaration (identifier) (parameter_list ...) (block ...))`. Only named nodes appear
  in the output, which is exactly what makes it a stable format for snapshot tests: reformatting
  whitespace or adding a comment doesn't change the S-expression.

```go
fmt.Println(fn.Text(src))
// func add(a, b int) int {
// 	return a + b
// }

fmt.Println(root.SExpr(lang))
// (source_file (package_clause (package_identifier)) (function_declaration ...))
```

## Finding nodes by position

Going from a byte offset or a `Point` (an editor cursor, a diagnostic location) down to the node
at that position is a common enough operation that it doesn't require a manual walk:

- `node.DescendantForByteRange(startByte, endByte uint32) *Node` — the smallest descendant that
  fully contains the given byte range.
- `node.NamedDescendantForByteRange(startByte, endByte uint32) *Node` — the same, but only
  considers named nodes (so you don't land on a stray `(` or `,`).
- `node.DescendantForPointRange(startPoint, endPoint Point) *Node` — the point-based equivalent,
  useful when you're starting from a `Row`/`Column` an editor gave you instead of a byte offset.

```go
identNode := root.NamedDescendantForByteRange(23, 27) // -> "café"'s content node
```

For the common case of "what node is under this single cursor position," `Tree` has convenience
wrappers that handle the zero-width range for you: `tree.NodeAtByte(offset)` and
`tree.NamedNodeAtByte(offset)` (mirrored on `Node` as `NodeAtByte`/`NamedNodeAtByte` when you're
already inside a subtree and don't want to search from the root). Reach for these instead of
constructing a one-byte range by hand.

## Next steps

For traversal patterns that scale to large files — and the actual reason `TreeCursor` exists
instead of just recursing on `Child`/`NamedChild` — see [Tree Cursors](/docs/tree-cursors).
Structural search across a tree (rather than positional lookup) is covered on the
[Queries](/docs/queries) page, and the query-built consumers —
[Syntax Highlighting](/docs/syntax-highlighting) and [Code Navigation](/docs/code-navigation) —
turn trees into styled ranges and symbol tables; grammar authoring and external scanners have
their own pages under [Authoring Languages](/docs/authoring-languages) and
[External Scanners](/docs/external-scanners).
