---
title: Migrating from smacker/go-tree-sitter
description: Move a CGo integration to the native gotreesitter v0.55.1 API.
nav_group: Using the Parser
order: 8
---

Version v0.55.1 does not contain `compat/smacker`. Do not use those import paths.
The [tagged source tree](https://github.com/odvcencio/gotreesitter/tree/v0.55.1)
contains the native Go API. A compatible import-only replacement is not verified.

## Use the native API

```sh
go get github.com/odvcencio/gotreesitter@v0.55.1
```

| Binding operation | Native gotreesitter operation |
|---|---|
| Select a Go grammar | `grammars.GoLanguage()` |
| Create a parser | `gotreesitter.NewParser(lang)` |
| Parse bytes | `parser.Parse(source)` |
| Read the root | `tree.RootNode()` |
| Read a node type | `node.Type(lang)` |
| Read node text | `node.Text(source)` |
| Find a field | `node.ChildByFieldName(name, lang)` |
| Release a tree | `tree.Release()` |

The native node API takes language and source arguments where needed.
Update callers and test tree shapes, queries, error inputs, and edit sequences.
See [Getting Started](/docs/getting-started), [Queries](/docs/queries), and
[Incremental Parsing](/docs/incremental-parsing) for the supported contracts.

The runtime requires no C compiler. Full-parse speed depends on the workload.
See [Performance](/docs/performance) for the dated evidence and v0.55.1 route default.
