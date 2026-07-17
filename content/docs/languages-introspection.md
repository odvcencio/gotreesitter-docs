---
title: Language Introspection
description: Enumerate a grammar's node types, fields, and supertype hierarchies at runtime — gotreesitter's answer to tree-sitter's static node-types.json.
nav_group: Languages
order: 2
---

Coming from tree-sitter, you enumerate a grammar's node types, fields, and supertypes by reading `node-types.json` — a static artifact the CLI emits next to `parser.c`. gotreesitter emits no such file and generates no typed-node Go code. Everything a `node-types.json` would tell you is a field or method on the loaded `*Language` you already parsed with, read at runtime.

That is a deliberate divergence, so state it plainly: **there is no `node-types.json` and no generated per-node Go struct.** The tradeoff buys uniformity — introspection works identically for a grammar you loaded dynamically with [`LoadLanguage`](/docs/languages) or registered from your own module with `RegisterExtension`, not only for the built-ins.

## Node types

`Language.SymbolNames []string` is every node type the grammar can produce, indexed by symbol ID; `Node.Type(lang)` is a lookup into it (`lang.SymbolNames[node.Symbol()]`). `SymbolByName` is the reverse:

```go
lang := grammars.GoLanguage()

len(lang.SymbolNames)                     // 233 symbol IDs for Go
lang.SymbolNames[86]                       // "identifier"
sym, ok := lang.SymbolByName("identifier") // (86, true)
```

`SymbolByName` builds a lookup map on first call and is O(1) after that; `(0, false)` means no such node type. Reach for it to resolve a name to a comparable `Symbol` once and match `node.Symbol()` in a hot loop instead of calling `Type` per node (see [Syntax Trees and Nodes](/docs/syntax-trees-and-nodes)), or to check a type exists before assembling a query string dynamically.

## Fields

`Language.FieldNames []string` is the field vocabulary, indexed by field ID, with index 0 reserved as `""` (no field). `FieldByName` is the reverse:

```go
lang.FieldNames[1]                         // "name"
fid, ok := lang.FieldByName("name")        // (1, true)
```

These back `Node.ChildByFieldName` and `Node.FieldNameForChild`. Enumerating the non-empty entries lists every field a grammar declares — "does this grammar even have a `body` field" is `lang.FieldByName("body")`.

## Supertypes

Some grammars group related node types under a supertype: Go's `_simple_type` covers `pointer_type`, `slice_type`, `map_type`, and nine more; `_statement` covers every statement form. Two methods expose the hierarchy:

- `IsSupertype(sym Symbol) bool` — whether a symbol is a supertype.
- `SupertypeChildren(sym Symbol) []Symbol` — the subtypes it expands to, or `nil` if `sym` isn't a supertype.

```go
super, _ := lang.SymbolByName("_simple_type")
lang.IsSupertype(super) // true
for _, sub := range lang.SupertypeChildren(super) {
    fmt.Println(lang.SymbolNames[sub]) // identifier, generic_type, qualified_type, ...
}
```

One caveat carries over from [Queries](/docs/queries): these tables power the `#has-ancestor?` predicate family, but the query compiler does **not** expand supertypes at pattern positions — `(_simple_type)` in a pattern matches only a node literally named `_simple_type`, never its members. When you want "any simple type," read the members here with `SupertypeChildren` and spell them out in an alternation `[...]`.

## Coming from node-types.json

| `node-types.json` | gotreesitter |
|---|---|
| the `"type"` inventory | `lang.SymbolNames` (`SymbolByName` for the reverse) |
| the `"fields"` object | `lang.FieldNames` (`FieldByName` for the reverse) |
| `"subtypes"` on a supertype | `lang.SupertypeChildren(sym)` (`IsSupertype` to detect one) |

The one thing the JSON gives that this surface does not is per-node generated Go types (the typed-node bindings some tree-sitter ecosystems ship). gotreesitter's node is always the untyped `*Node`; you narrow it with `Symbol()`/`Type()` and the tables above, not with a generated struct.

## Next steps

- [Languages](/docs/languages) — the registry and what a loaded `*Language` carries.
- [Queries](/docs/queries) — why supertypes don't expand at pattern positions.
- [Syntax Trees and Nodes](/docs/syntax-trees-and-nodes) — `Symbol()`, `Type()`, and field lookups on a node instance.
