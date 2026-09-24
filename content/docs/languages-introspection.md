---
title: Language Introspection
description: Enumerate a grammar's node types, fields, and supertype hierarchies at runtime — gotreesitter's answer to tree-sitter's static node-types.json.
nav_group: Languages
order: 2
---

Coming from tree-sitter, you enumerate a grammar's node types, fields, and supertypes by reading
`node-types.json` — a static artifact the CLI emits next to `parser.c`. gotreesitter emits no
such file and generates no typed-node Go code. Everything a `node-types.json` would tell you is a
field or method on the loaded `*Language` you already parsed with, and you read it at runtime.

State the divergence plainly: **there is no `node-types.json` and no generated per-node Go
struct.** The tradeoff buys uniformity — introspection works identically for a grammar you loaded
dynamically with [`LoadLanguage`](/docs/languages) or registered from your own module with
`RegisterExtension`, not only for the built-ins.

## Node types

`Language.SymbolNames []string` lists every node type the grammar can produce, indexed by symbol
ID. `Node.Type(lang)` looks up a value in it (`lang.SymbolNames[node.Symbol()]`). `SymbolByName`
runs the reverse lookup:

```go
lang := grammars.GoLanguage()

fmt.Println(len(lang.SymbolNames))
sym, ok := lang.SymbolByName("identifier")
if ok {
    fmt.Println(sym, lang.SymbolNames[sym])
}


```

`SymbolByName` builds a lookup map on first call and runs O(1) after that; `(0, false)` means no
such node type exists. Reach for it to resolve a name to a comparable `Symbol` once, then match
`node.Symbol()` in a hot loop instead of calling `Type` per node (see
[Syntax Trees and Nodes](/docs/syntax-trees-and-nodes)), or to check a type exists before
assembling a query string dynamically.

## Fields

`Language.FieldNames []string` lists the field vocabulary, indexed by field ID, with index 0
reserved as `""` (no field). `FieldByName` runs the reverse lookup:

```go
fid, ok := lang.FieldByName("name")
if ok {
    fmt.Println(fid, lang.FieldNames[fid])
}


```

These tables back `Node.ChildByFieldName` and `Node.FieldNameForChild`. Enumerating the
non-empty entries lists every field a grammar declares — to check whether a grammar even has a
`body` field, call `lang.FieldByName("body")`.

## Supertypes

Some grammars group related node types under a supertype: Go declares `_simple_type` and `_statement`. Inspect the loaded table for their members.
Two methods expose the hierarchy:

- `IsSupertype(sym Symbol) bool` — reports whether a symbol is a supertype.
- `SupertypeChildren(sym Symbol) []Symbol` — returns the subtypes it expands to, or `nil` if
  `sym` is not a supertype.

```go
super, _ := lang.SymbolByName("_simple_type")
lang.IsSupertype(super) // true
for _, sub := range lang.SupertypeChildren(super) {
    fmt.Println(lang.SymbolNames[sub]) // identifier, generic_type, qualified_type, ...
}
```

Supertype patterns use these tables and the node's hidden supertype metadata.
A `supertype/subtype` pattern also checks the subtype. See [Queries](/docs/queries).
The [dated parity boards](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/docs/c-parity-boards.md)
record map differences. Verify the grammar and query you need.

## Coming from node-types.json

| `node-types.json` | gotreesitter |
|---|---|
| the `"type"` inventory | `lang.SymbolNames` (`SymbolByName` for the reverse) |
| the `"fields"` object | `lang.FieldNames` (`FieldByName` for the reverse) |
| `"subtypes"` on a supertype | `lang.SupertypeChildren(sym)` (`IsSupertype` to detect one) |

The JSON file gives you one thing this surface does not: per-node generated Go types (the
typed-node bindings some tree-sitter ecosystems ship). gotreesitter's node is always the untyped
`*Node`; you narrow it with `Symbol()`/`Type()` and the tables above, not with a generated struct.

## Next steps

- [Languages](/docs/languages) — the registry and what a loaded `*Language` carries.
- [Queries](/docs/queries) — why supertypes don't expand at pattern positions.
- [Syntax Trees and Nodes](/docs/syntax-trees-and-nodes) — `Symbol()`, `Type()`, and field lookups on a node instance.
