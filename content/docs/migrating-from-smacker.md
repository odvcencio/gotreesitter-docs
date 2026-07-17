---
title: Migrating from smacker/go-tree-sitter
description: Move off the unmaintained smacker/go-tree-sitter cgo binding with an import swap, using the compat/smacker shim — no per-node rewrites.
nav_group: Using the Parser
order: 8
---

[`github.com/smacker/go-tree-sitter`](https://github.com/smacker/go-tree-sitter) is a cgo
binding to the C tree-sitter runtime, and it has been unmaintained since **August 2024**. Plenty
of tools still depend on it — language servers, linters, security scanners — often because
rewriting every call site that touches a `*sitter.Node` is more work than anyone wants to sign
up for. `compat/smacker` is the drop-in path off that dead dependency: swap two imports, keep
every other line of your code.

## The import swap

```go
// before
import sitter "github.com/smacker/go-tree-sitter"
import "github.com/smacker/go-tree-sitter/golang"

// after
import sitter "github.com/odvcencio/gotreesitter/compat/smacker"
import "github.com/odvcencio/gotreesitter/compat/smacker/golang"
```

That's the whole migration for most call sites. Everything downstream of `sitter.ParseCtx`,
`node.Type()`, `node.ChildByFieldName(...)`, and `query.CaptureNameForId(...)` keeps compiling
and behaving the same way, because the shim re-exposes smacker's exact method shapes.

## Why a wrapper, not a type alias

smacker's `Node` methods take no language argument — the C node carries its own language
pointer internally, so `node.Type()` alone is enough to resolve a name. gotreesitter's node is a
pure-Go, arena-allocated value that doesn't embed a language pointer; every native gotreesitter
method threads a `*gotreesitter.Language` explicitly instead. That difference means a straight
`type Node = gotreesitter.Node` re-export can't work — the method signatures don't match. The
shim closes that gap by wrapping each node together with the `*Language` (and source bytes) it
was parsed with, then re-exposing smacker's argument-free surface on top. Constructing a shim
node is what threads the language through once, at parse time, so every call after that reads
exactly like smacker.

## API equivalence

The call sites that matter for a typical consumer are unchanged:

- `sitter.ParseCtx(ctx, content, lang)` and the stateful `Parser`/`Tree` pair
  (`NewParser`, `SetLanguage`, `ParseCtx`, `Parse`, `RootNode`, `Edit`, `Copy`).
- `Node` methods: `Type()`, `Content(src)`, `String()`, `ChildByFieldName(name)`,
  `FieldNameForChild`, `Child`, `NamedChild`, `ChildCount`, `NamedChildCount`, `Parent`,
  `IsNamed`, `IsNull`, `StartByte`, `EndByte`, `StartPoint`, `EndPoint`, `Range`, `Equal`.
- `Query`/`QueryCursor`: `NewQuery`, `NewQueryCursor`, `Exec`, `NextMatch`, and
  `query.CaptureNameForId(capture.Index)` for turning a match capture back into its query name.

`Close()` on `Parser`, `Tree`, `Query`, and `QueryCursor` is a no-op in the shim — gotreesitter
holds no C resources, so there's nothing to release, but the call stays valid so you don't have
to strip it out of existing defer chains.

## The build and CI win

The reason to make this swap isn't a speed claim — it's that a cgo binding breaks release
matrices smacker never had to support. gotreesitter is pure Go, so the same build works
everywhere Go's toolchain does, with no C compiler in the loop:

```sh
# a WASI/WASM target and a Linux/arm64 target — both cgo-impossible, both clean here
GOOS=wasip1 GOARCH=wasm CGO_ENABLED=0 go build ./...
GOOS=linux  GOARCH=arm64 CGO_ENABLED=0 go build ./...
```

No C toolchain to install or pin in CI, no per-platform shared library to vendor, and one
static binary per target instead of a matrix of cgo cross-compiles. `go test -race` also sees
the whole call path, since there's no C frontier for the race detector to lose visibility across.

On raw parsing throughput, gotreesitter's canonical real-code full-parse geomean is **5.48× C**
— slower, not faster, on that workload; see [Performance](/docs/performance) for the full
methodology and fleet distribution. Where gotreesitter does win outright is allocations on the
incremental path: reparsing after a small edit is zero-allocation, discussed in
[Incremental Parsing](/docs/incremental-parsing). Migrate for portability and the dead upstream,
not for a full-parse speed win you won't see.

## Per-grammar subpackages

Every grammar smacker shipped a subpackage for has a `compat/smacker/<language>` counterpart
that mirrors the same layout — `compat/smacker/golang`, `compat/smacker/python`,
`compat/smacker/rust`, and so on — each exposing the same `GetLanguage()` function smacker's
subpackages export. If a grammar you depend on isn't covered yet, adding one is a few lines
mapping the name to gotreesitter's existing `grammars.XLanguage()` entry for that language.

> [!NOTE] Availability
> `compat/smacker` lands in gotreesitter's module — once released, there's no separate `go get`,
> just the one import swap above. It is not in `v0.38.0`; until the release that carries it is
> tagged, pin the module to the revision that includes `compat/smacker` (or track `main`). See
> [Getting Started](/docs/getting-started) if you're setting up gotreesitter for the first time
> rather than migrating an existing smacker integration.
