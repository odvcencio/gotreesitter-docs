---
title: v0.54.0 Release
description: Release scope, API additions, and behavior changes shipped on 2026-09-23.
nav_group: Project
order: 3
---

Version **v0.54.0** was released on **2026-09-23**. The site and playground pin that module.
The [release commit](https://github.com/odvcencio/gotreesitter/commit/90a9c277a928aca1f100f170c57b869c4686f068),
[tagged changelog](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/CHANGELOG.md),
and [roadmap](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/docs/roadmap.md)
are the sources for this release.

```sh
go get github.com/odvcencio/gotreesitter@v0.54.0
```

## Behavior changes

- Production GLR parsing is the default. `GTS_ADMISSION_CANDIDATE` now defaults off.
  Set it to `1`, `true`, `on`, or `yes` to enable compact parsing.
- Incremental C and Java token sources retain valid queued literal-tail tokens.
- Plain `ParseIncremental` now arms the reuse-budget stop.
- A changed source length without a recorded `Tree.Edit` causes a fresh parse.
- Groovy incremental calls use a fresh full parse.
- Compact reuse proofs exempt extra leaves. Other leaves still need a proof.
- GLR shape-prefix cache invalidation occurs on a link-0 rewrite.
- Explicit lexer end-token acceptance, named immediate-token precedence, and YAML
  bare-opener error shapes are corrected.

Compact parser graduation remains incomplete. The release notes retain the COBOL
column-dependency over-invalidation gap. The Python list-splat issue #1274 and duplicate
escape-sequence issue #1275 are outside this tag. No patch fix is claimed here.

## Exported API changes from v0.53.0

This list comes from `git diff v0.53.0 v0.54.0 -- '*.go'`, including build-tagged code.
Generated blob hash changes and methods on private types are not new consumer entry points.

| Surface | Change | Source |
|---|---|---|
| Facts | `FactProgram.ExtractInto(tree, dst)` reuses caller-owned `FactSet` slices. | [fact_program.go](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/fact_program.go) |
| Queries | `QueryCapture.ByteRange`, `PointRange`, and `Range` return effective capture spans, including `#offset!`. | [query.go](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/query.go) |
| Injections | `InjectionParser.SetTimeoutMicros` and `SetCancellationFlag` apply to parent and child parsers. | [injection.go](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/injection.go) |
| Blob metadata | `Language.BlobInfo`, `LanguageBlobInfo`, `BlobRuntimeVersion`, `WrapLanguageBlobVersionHeader`, and `UnwrapLanguageBlobVersionHeader` expose version headers. | [header API](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/language_blob_version_header.go) |
| Blob encoding | `EncodeLanguageBlobWithGenerator` and `DefaultBlobGeneratorVersion`; `EncodeLanguageBlob` now emits a version header. | [encoder](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/language_blob_encode.go) |
| Blob loading | `MaxDecompressedBlobSize` and `ErrDecompressedBlobTooLarge` expose the 64 MiB default decompression limit. | [loader](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/load_language.go) |
| Registry loading | `grammars.DecodeAndCertifyLanguageBlob` and its `grammars/runtime` counterpart use the embedded repair and certification path. | [registry loader](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/grammars/embedded_loader.go) |
| Lexer tables | `LexState.AcceptEOF` records explicit end-token acceptance. | [language.go](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/language.go) |
| Scanner ports | Scanner structs bind to a language through `ExternalScannerForLanguage`. Stub builds add `RegisterCaddySupport`, `RegisterDisassemblySupport`, and `RegisterNimSupport`. | [scanner sources](https://github.com/odvcencio/gotreesitter/tree/v0.54.0/grammars/runtime) |
| Nil accessors | Scalar node accessors return zero or false on nil; `Tree.Source`, `Language`, and `Edits` return nil. | [tree.go](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/tree.go) |

See [Code Navigation](/docs/code-navigation), [Queries](/docs/queries),
[Language Injection](/docs/language-injection), and [Authoring Languages](/docs/authoring-languages)
for use and ownership rules.

## Diagnostic additions

The diff also adds `ResetAdmissionCandidateCounters`, `CRecoverEOFBareRootReceipted`, and
`CRecoverEOFBareRootReceiptedNames`. These expose route counters and certified recovery names.
See [admission_switch.go](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/admission_switch.go)
and [parser_recover_c.go](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/parser_recover_c.go).

`PerfCounters` adds `GSSCanReachVisits`, `ShapePrefixWalkSteps`, `ShapePrefixEpochBumps`,
`ProbeLexBytes`, and `ProbeLexTokens`.
`DiagnosticParserCoreGenericWork` adds `ZeroWidthCatchUpMissedMerge`.
The compact EOF admission API adds `ScannerProbes` to `EOFRecoveryAdmissionWork` and
`Mechanism`, `ScannerQuiescenceProved`, `ScannerQuiescenceStateless`,
`ScannerQuiescenceStates`, `ExternalCount`, and `ExternalDigest` to `EOFRecoveryAdmissionReceipt`.
It adds the mechanism constants `EOFRecoveryAdmissionMechanismScannerFree` and
`EOFRecoveryAdmissionMechanismScannerQuiescent`.
These surfaces can depend on build tags. Read the
[counter source](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/perf_counters.go),
[diagnostic work record](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/parsercore_phase0_driver.go),
and [EOF contract](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/parsercore_phase0_eof_recovery_admission_contract.go)
before using them.

## Counts and measurements

The registry remains at 206 grammars. The language guide lists 119 external scanners and
seven token-source implementations. These are different counts.
See [Languages](/docs/languages) for the catalog,
[Recovery and Correctness](/docs/recovery-and-correctness) for dated parity results, and
[Performance](/docs/performance) for the release comparison and benchmark limits.
