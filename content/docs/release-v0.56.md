---
title: "v0.56.0 Release"
description: What changed in gotreesitter v0.56.0 (2026-10-13), with pull requests and the publication exception.
nav_group: Project
order: 3
---

This page summarizes gotreesitter v0.56.0, released on 2026-10-13. The tagged
[CHANGELOG](https://github.com/odvcencio/gotreesitter/blob/v0.56.0/CHANGELOG.md) is the source
for every entry and its linked evidence.

```sh
go get github.com/odvcencio/gotreesitter@v0.56.0
```

## Added

- Opt-in declaration, signature and call-argument facts for Go, Python, JavaScript, TypeScript
  and TSX through `NewFactProgramWithOptions`, `FactDeclarations`, `FactSignatures` and
  `FactCallArguments`
  ([#1435](https://github.com/odvcencio/gotreesitter/issues/1435)). The existing
  `NewFactProgram`, `Definitions` and `FactAll` keep their contracts. Function literals and
  function types have no signature facts. See [Code Navigation](/docs/code-navigation).

## Changed

- Bounded lexer-read certificates for incremental subtree reuse
  ([#1402](https://github.com/odvcencio/gotreesitter/pull/1402)).
- Certified incremental reuse for Go, Java, JavaScript, TypeScript, Python, Rust, C and C++
  ([#1413](https://github.com/odvcencio/gotreesitter/pull/1413)), and for ASCII run edits in Lua,
  Nickel, Starlark, Properties and FIRRTL
  ([#1412](https://github.com/odvcencio/gotreesitter/pull/1412)).
- Bounded external-scanner checkpoint memory growth
  ([#1410](https://github.com/odvcencio/gotreesitter/pull/1410)).
- Certified subtree reuse for large edits without a redundant fresh parse
  ([#1421](https://github.com/odvcencio/gotreesitter/pull/1421)).
- Lower compact-route runtime and acceptance costs
  ([#1434](https://github.com/odvcencio/gotreesitter/pull/1434)), and cheaper scanner checkpoints
  for owned YAML parses
  ([#1446](https://github.com/odvcencio/gotreesitter/pull/1446),
  [#1447](https://github.com/odvcencio/gotreesitter/pull/1447),
  [#1448](https://github.com/odvcencio/gotreesitter/pull/1448)).

## Fixed

- An earlier TSX generic call stays intact when a typed function is appended after it
  ([#1443](https://github.com/odvcencio/gotreesitter/issues/1443)).
- YAML syntax errors and recovered nodes survive result normalization, and `Node.SExpr` includes
  `ERROR` roots and children ([#1400](https://github.com/odvcencio/gotreesitter/issues/1400)).
- Pruned parses never publish an incremental-reuse certificate
  ([#1440](https://github.com/odvcencio/gotreesitter/pull/1440)).
- Go type declarations, aliases and explicit interface methods appear in inferred tags and
  outlines ([#1438](https://github.com/odvcencio/gotreesitter/pull/1438)).
- C recovery state survives edit sessions
  ([#1416](https://github.com/odvcencio/gotreesitter/pull/1416)); locked-C parity for AWK
  ([#1420](https://github.com/odvcencio/gotreesitter/pull/1420)) and for conflict ordering and
  scanner behavior in TypeScript, Bash, C++, C#, Swift, Dart and SQL
  ([#1414](https://github.com/odvcencio/gotreesitter/pull/1414)).
- Incremental verification matches fresh stop reasons
  ([#1409](https://github.com/odvcencio/gotreesitter/pull/1409)), keeps verified fresh results
  ([#1408](https://github.com/odvcencio/gotreesitter/pull/1408)), preserves deadlines and
  verifier errors ([#1426](https://github.com/odvcencio/gotreesitter/pull/1426)).
- `HasError()` stays true when hidden missing tokens are removed from the public tree
  ([#1419](https://github.com/odvcencio/gotreesitter/pull/1419)).

## Maintenance

- Historical AWK raw-tree receipts are labeled as historical
  ([#1422](https://github.com/odvcencio/gotreesitter/pull/1422),
  [#1424](https://github.com/odvcencio/gotreesitter/pull/1424)).
- Required locked-C CI covers scanner and lexer regressions and JavaScript and TypeScript edit
  sessions ([#1425](https://github.com/odvcencio/gotreesitter/pull/1425)).

## Publication exception

On 2026-10-04, the owner authorized the v0.56.0-only tag-creation exception. All other gates
remain mandatory under the
[release process](https://github.com/odvcencio/gotreesitter/blob/v0.56.0/docs/releasing.md#v0560-only-tag-creation-exception).

## Measurement

The v0.56.0 changelog carries no new benchmark receipt, and the tag's
[BENCH.md](https://github.com/odvcencio/gotreesitter/blob/v0.56.0/BENCH.md) still reads
"Measured 2026-09-27". The numbers on [Performance](/docs/performance) are that dated control,
taken on the v0.55.1 source. They are not v0.56.0 timings.
