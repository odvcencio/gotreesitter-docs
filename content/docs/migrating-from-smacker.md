---
title: Migrating from smacker/go-tree-sitter
description: Plan a future move off the unmaintained smacker/go-tree-sitter cgo binding. The compat/smacker shim remains unreleased.
nav_group: Using the Parser
order: 8
---

[`github.com/smacker/go-tree-sitter`](https://github.com/smacker/go-tree-sitter) is a cgo binding
to the C tree-sitter runtime, and it has been unmaintained since **August 2024**. Plenty of tools
still depend on it — language servers, linters, security scanners — often because rewriting every
call site that touches a `*sitter.Node` is more work than anyone wants to sign up for.
`compat/smacker` is the planned drop-in path off that dead dependency. It remains unreleased and
is not part of v0.48.0. Treat the import swap below as an evaluated migration design, not a
stable module contract.

> [!CAUTION] Release status
> Do not depend on `compat/smacker` from a tagged gotreesitter release yet. Use the native API, or
> test an exact source revision outside a production dependency policy.

## The import swap

```go
// before
import sitter "github.com/smacker/go-tree-sitter"
import "github.com/smacker/go-tree-sitter/golang"

// after
import sitter "github.com/odvcencio/gotreesitter/compat/smacker"
import "github.com/odvcencio/gotreesitter/compat/smacker/golang"
```

That swap is the whole migration for most call sites. Everything downstream of
`sitter.ParseCtx`, `node.Type()`, `node.ChildByFieldName(...)`, and
`query.CaptureNameForId(...)` keeps compiling and behaving the same way, because the shim
re-exposes smacker's exact method shapes.

## Why a wrapper, not a type alias

smacker's `Node` methods take no language argument — the C node carries its own language pointer
internally, so `node.Type()` alone is enough to resolve a name. gotreesitter's node is a pure-Go,
arena-allocated value that does not embed a language pointer; every native gotreesitter method
threads a `*gotreesitter.Language` explicitly instead. That difference means a straight
`type Node = gotreesitter.Node` re-export cannot work — the method signatures do not match. The
shim closes that gap by wrapping each node together with the `*Language` (and source bytes) it
was parsed with, then re-exposing smacker's argument-free surface on top. Constructing a shim
node threads the language through once, at parse time, so every call after that reads exactly
like smacker.

## API equivalence

The call sites that matter for a typical consumer stay unchanged:

- `sitter.ParseCtx(ctx, content, lang)` and the stateful `Parser`/`Tree` pair
  (`NewParser`, `SetLanguage`, `ParseCtx`, `Parse`, `RootNode`, `Edit`, `Copy`).
- `Node` methods: `Type()`, `Content(src)`, `String()`, `ChildByFieldName(name)`,
  `FieldNameForChild`, `Child`, `NamedChild`, `ChildCount`, `NamedChildCount`, `Parent`,
  `IsNamed`, `IsNull`, `StartByte`, `EndByte`, `StartPoint`, `EndPoint`, `Range`, `Equal`.
- `Query`/`QueryCursor`: `NewQuery`, `NewQueryCursor`, `Exec`, `NextMatch`, and
  `query.CaptureNameForId(capture.Index)` for turning a match capture back into its query name.

`Close()` on `Parser`, `Tree`, `Query`, and `QueryCursor` is a no-op in the shim — gotreesitter
holds no C resources, so it has nothing to release, but the call stays valid so you do not have
to strip it out of existing defer chains.

## The build and CI win

The reason to make this swap is not a speed claim — it is that a cgo binding breaks release
matrices smacker never had to support. gotreesitter is pure Go, so the same build works
everywhere Go's toolchain does, with no C compiler in the loop:

```sh
# a WASI/WASM target and a Linux/arm64 target — both cgo-impossible, both clean here
GOOS=wasip1 GOARCH=wasm CGO_ENABLED=0 go build ./...
GOOS=linux  GOARCH=arm64 CGO_ENABLED=0 go build ./...
```

There is no C toolchain to install or pin in CI, no per-platform shared library to vendor, and
one static binary per target instead of a matrix of cgo cross-compiles. `go test -race` also sees
the whole call path, since there is no C frontier for the race detector to lose visibility
across.

The sealed v0.45.0 production route receipt is **5.526× C** on the real-code matrix. The compact
route has a separate **2.9975× C** receipt. Version 0.48 selects compact parsing only for eligible
fresh full parses and uses production parsing after a compact decline. See
[Performance](/docs/performance) for the route-specific evidence. Migrate for portability and for
the dead upstream, not for a full-parse speed win you will not see.

## Per-grammar subpackages

When it ships, every grammar smacker shipped will have a `compat/smacker/<language>` counterpart.
It will mirror the same layout — `compat/smacker/golang`, `compat/smacker/python`,
`compat/smacker/rust`, and similar packages. Each package will expose smacker's `GetLanguage()`
function. Do not rely on that surface until a compatibility release publishes it.

> [!NOTE] Availability
> `compat/smacker` does not ship in v0.48.0. Do not assume its import paths exist in a tagged
> module. Wait for an announced compatibility release before you make the swap in production. See
> [Getting Started](/docs/getting-started) if you are setting up gotreesitter for the first time.
