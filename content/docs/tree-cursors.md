---
title: Tree Cursors
description: Efficient stack-based traversal with TreeCursor and the Walk helper, and when to reach for each over plain Node navigation.
nav_group: Using the Parser
order: 2
---

[Syntax Trees and Nodes](/docs/syntax-trees-and-nodes) covers `Child`, `NamedChild`, `Parent`, and
friends — enough to write a recursive walk by hand. For traversing a whole tree, gotreesitter gives
you two purpose-built tools instead: the package-level `Walk` function, and the stateful
`TreeCursor` type. Both avoid re-deriving position information as they move; which one you want
depends on how much control you need over the walk.

## `Walk`: the default for a full traversal

`gotreesitter.Walk` is a free function that does a depth-first, pre-order traversal of a subtree,
calling your function once per node:

```go
func Walk(node *Node, fn func(node *Node, depth int) WalkAction)
```

`WalkAction` lets the callback control descent: `WalkContinue` (visit children and siblings as
normal), `WalkSkipChildren` (skip this node's children, keep going to siblings), or `WalkStop` (end
the whole walk immediately).

```go
count := 0
gts.Walk(root, func(n *gts.Node, depth int) gts.WalkAction {
    count++
    if n.Type(lang) == "parameter_list" {
        return gts.WalkSkipChildren // don't descend into parameter lists
    }
    return gts.WalkContinue
})
```

`Walk` maintains its own explicit stack internally (pulled from a `sync.Pool`, so repeated calls
don't allocate a fresh stack each time) rather than recursing — that's what makes it safe on very
deep trees where naive `Child`-recursion would grow the Go call stack proportionally to nesting
depth. For "visit every node, optionally skip a subtree, optionally bail early," `Walk` is the
right default and the least code to write.

## `TreeCursor`: stateful, bidirectional navigation

`TreeCursor` is what you want when a single forward pass isn't enough — when you need to move back
up to a parent, jump to a specific field, or resume a walk from wherever you left off. It's built
around an explicit stack of `(node, childIndex)` frames:

```go
c := gts.NewTreeCursorFromTree(tree) // starts at tree.RootNode()
```

To start somewhere other than the root, use `NewTreeCursor(node, tree) *TreeCursor` with an
arbitrary starting node instead. In both constructors the `tree` argument is optional (`nil` is
safe) but enables `CurrentNodeType`, `CurrentNodeText`, and field-name lookups, which need the
language and source to resolve names.

### Movement

| Method | Moves to |
|---|---|
| `GotoFirstChild()` / `GotoLastChild()` | first / last child (any kind) |
| `GotoNextSibling()` / `GotoPrevSibling()` | next / previous sibling |
| `GotoParent()` | parent node |
| `GotoFirstNamedChild()` / `GotoLastNamedChild()` | first / last **named** child |
| `GotoNextNamedSibling()` / `GotoPrevNamedSibling()` | next / previous **named** sibling |
| `GotoChildByFieldName(name)` / `GotoChildByFieldID(id)` | first child with that field |
| `GotoFirstChildForByte(b)` / `GotoFirstChildForPoint(p)` | first child whose span contains `b`/`p` |

Every `Goto*` method returns `bool` (false leaves the cursor where it was) except the byte/point
variants, which return the matched child's index as `int64`, or `-1` if nothing matched.

### Reading the current position

`CurrentNode() *Node`, `CurrentNodeType() string`, `CurrentNodeText() string`,
`CurrentNodeIsNamed() bool`, `CurrentFieldName() string`, `CurrentFieldID() FieldID`, and
`Depth() int` (0 at the cursor's starting node) all read the cursor's current frame without moving
it. `Reset(node)` and `ResetTree(tree)` rewind the cursor to a new starting point, and `Copy()`
clones the whole frame stack so you can fork a traversal — branch off to explore a subtree in a
helper function and come back to the original cursor untouched.

### Why a cursor instead of `Parent()`/`NextSibling()`?

`Node.Parent()` and `Node.NextSibling()` work by consulting parent links stored on the tree's
nodes. Those links aren't wired eagerly — the *first* call to `Parent()`, `NextSibling()`, or
`PrevSibling()` anywhere in a tree pays a one-time cost to wire parent links for the whole tree, so
subsequent calls are cheap. `TreeCursor` never touches that mechanism at all: `GotoParent()` just
pops the cursor's own frame stack, and `GotoNextSibling()` re-indexes into the parent frame it's
already holding a pointer to. For code that's going to walk a large tree anyway, a cursor sidesteps
the wiring step entirely and gives you backward movement, field-aware descent, and byte/point
anchoring that plain `Child`/`NamedChild` recursion doesn't have equivalents for.

### A manual depth-first walk

`Walk` covers the common case, but sometimes you need the traversal inlined into a larger loop
(a hand-written visitor, a generator, code that needs to interleave tree movement with other
state). The classic cursor-based DFS — descend, and when you can't, advance to the next sibling or
climb until you can:

```go
c := gts.NewTreeCursorFromTree(tree)
count := 0
for {
    count++ // visit c.CurrentNode() here
    if c.GotoFirstChild() {
        continue
    }
    for {
        if c.GotoNextSibling() {
            break
        }
        if !c.GotoParent() {
            return count // back at the root with nowhere left to go
        }
    }
}
```

This visits nodes in the same order as `Walk` — it's a lower-level building block, not a faster
one.

### Field-aware, named-only descent

Cursors compose named-only movement with field lookups, which is useful for grammar-shape-specific
extraction (walking every top-level declaration's name and body without caring about punctuation):

```go
c := gts.NewTreeCursorFromTree(tree)
for ok := c.GotoFirstNamedChild(); ok; ok = c.GotoNextNamedSibling() {
    fmt.Printf("%s at byte %d\n", c.CurrentNodeType(), c.CurrentNode().StartByte())
}

c.Reset(tree.RootNode())
if c.GotoChildByFieldName("body") {
    fmt.Println(c.CurrentFieldName(), c.CurrentNodeType()) // "body block"
}
```

## Which one to use

- **One-off lookup, small subtree, or you only need forward `Child`/`NamedChild` access** — plain
  `Node` navigation is the least ceremony; see
  [Syntax Trees and Nodes](/docs/syntax-trees-and-nodes).
- **Visit every node (optionally skipping subtrees or stopping early), no need to move backward** —
  `gotreesitter.Walk`. It's the default for highlighting, linting passes, and extraction that
  process a whole tree once.
- **You need to move up as well as down, jump to a field, anchor to a byte/point, or fork a
  traversal mid-walk** — `TreeCursor`.

## Lifecycle

A `TreeCursor` holds direct pointers into a tree's nodes, the same as any `*Node` you keep around.
Recreate the cursor after `Tree.Release()`, `Tree.Edit()`, or an incremental reparse that produces
a new tree — don't keep using a cursor built from a tree that's since changed underneath it. A
cursor is not safe to share across goroutines without your own synchronization; `Copy()` gives each
goroutine its own frame stack over the same underlying tree if you need to parallelize a walk.
