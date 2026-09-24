# v0.54.0 documentation audit

Audit date: 2026-09-24. Base: docs commit `bd8486e` (PR #21).
Release source: gotreesitter tag `v0.54.0`, commit
[`90a9c277a928aca1f100f170c57b869c4686f068`](https://github.com/odvcencio/gotreesitter/commit/90a9c277a928aca1f100f170c57b869c4686f068).
The release date is 2026-09-23. PR [#1273](https://github.com/odvcencio/gotreesitter/pull/1273)
records the release notes and roadmap update.

Read scope: repository README, all 20 existing `content/docs` pages, landing and changelog
components, global version banner, playground and authoring shells, slide deck, build scripts,
module pins, and release catalog. The exported API review used
`git diff v0.53.0 v0.54.0 -- '*.go'` from a separate tag-only clone inside this worktree.
No owner checkout was used. No parser benchmark was run to create a new public claim.

## Stale statements and corrections

| Surface | Stale or unsupported statement | Correction and evidence |
|---|---|---|
| `go.mod`, README, install examples | Site used v0.53.0; install commands selected an unspecified version. | Pin v0.54.0 in the module and commands. [Release README](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/README.md). |
| Changelog catalog and latest-release card | Catalog ended at v0.53.0; card still described v0.48. | Load the exact v0.54.0 changelog and its three archives, with checksums and per-file source lines. [Changelog](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/CHANGELOG.md). |
| Landing, introduction, architecture, migration | Compact parsing described as the default for eligible fresh parses. | Production is the default; compact requires `GTS_ADMISSION_CANDIDATE=1`. Graduation remains incomplete. [Roadmap](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/docs/roadmap.md). |
| Landing edit pill and timing cards | Zero edited allocations; 1.98 µs, 9.9 ns, 5,500×, and 1.1 million× presented as current. | Use the dated 2026-09-23 control: 439,825 ns, 410 B, 5 allocations for edits; 27.4 ns, zero allocations for no-edit. Name revision `4637be52a` and host. [BENCH](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/BENCH.md#current-quiet-host-receipt). |
| Performance, introduction, incremental | Historical table combined a 10.907 ms full parse with July edit results; undated data could appear current. | Remove the mixed table and derived multipliers. Use dated, revision-specific results. [BENCH](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/BENCH.md). |
| Performance, landing, introduction, migration | v0.45.0 ratios 5.526× / 2.9975× treated as current complete evidence. | Use sealed v9 (2026-08-02): 4.815× production / 3.986× compact. Retain old values only as history. Include the source's method and limits. [Sealed v9](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/BENCH.md#sealed-epoch--v9-hardware-attested-authoritative). |
| Performance and landing feature | v0.53.0 127.5 µs edit result described the current release. | Label that paired comparison as historical and host-specific. Add the nine-language v0.54.0 default-route comparison without an invented aggregate. [Dated release notes](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/CHANGELOG.md#0540---2026-09-23). |
| Performance, introduction, recovery, architecture | Old fleet, memory, and campaign figures lacked a clear current measurement scope. | Remove those headline figures. Retain the dated benchmark and parity tables with explicit scope. [BENCH](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/BENCH.md), [boards](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/docs/c-parity-boards.md). |
| Banner, landing, introduction, recovery | 206/206 could be read as parity for all inputs; highlight floor 200 could be read as a result. | Distinguish the curated gate thresholds from the dated boards. [Ratchet](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/cgo_harness/parity_gate_ratchet_test.go). |
| Recovery | No current dated board table. | Add query 101/103, highlight 204/206 with two missing C references, supertype maps 40/69 (2026-09-08), and recovery 39/79 (2026-09-19). These are published board results, not an exact-tag rerun. [Boards](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/docs/c-parity-boards.md). |
| Languages, recovery | 124 elected languages, 203/3 blob split, and 273 extension mappings presented as current. | Remove unverified aggregates. Describe how to inspect the loaded registry. Keep the published 206 grammars, 119 scanners, and seven token-source implementations. [Language guide](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/docs/languages.md). |
| Incremental | Missing current fallback behavior; same-pointer result described without the retained handle. | Document missing-edit length fallback, Groovy full parse, C/Java queued-token resume, reuse-budget stop, and handle ownership. [Release notes](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/CHANGELOG.md), [tree API](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/tree.go). |
| Highlighting, navigation | Release the old tree only if its pointer differs. | Release the old handle even on same-pointer returns; calculate changed ranges before release. [Tree ownership](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/tree.go). |
| Architecture | Same-width edit path described as zero-allocation. | Only the dated no-edit lane has zero allocations. [BENCH](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/BENCH.md). |
| Queries, introspection | `MISSING` and supertype patterns described as unsupported. | Describe supported matching and known metadata limits. [Compiler](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/query_compile.go), [boards](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/docs/c-parity-boards.md). |
| Queries | `#offset!` described as stored but not applied; capture range API absent. | Document effective capture ranges and `ByteRange`, `PointRange`, `Range`, and `Text`. [Query API](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/query.go), [offset implementation](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/query_offset.go). |
| Introspection | Fixed Go symbol count and IDs presented as stable. | Resolve IDs by name and enumerate loaded tables. [Language API](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/language.go). |
| Code navigation | No `FactProgram.ExtractInto` contract. | Add replacement, capacity, language identity, and concurrency rules. [Fact API](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/fact_program.go). |
| Injection | No timeout or cancellation API. | Add per-parse limits for parent and child parsers, including cached parsers. [Injection API](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/injection.go). |
| Trees | New nil-accessor behavior absent. | List the accessors and their zero/nil return values; keep release lifetime rules. [Tree API](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/tree.go). |
| Authoring, languages | Blobs described only as gzip+gob; no runtime-version or decompression-limit contract. | Document version headers, generator identity, `BlobInfo`, legacy support, 64 MiB limit, and error sentinel. [Header](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/language_blob_version_header.go), [loader](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/load_language.go). |
| Languages | Embedded certification entry point absent. | Document `DecodeAndCertifyLanguageBlob` without promising parity for arbitrary blobs. [Registry loader](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/grammars/runtime/embedded_loader.go). |
| Authoring | `RegisterExtension` said to lack `TagsQuery`. | Correct the field contract. [Registry](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/grammars/registry.go). |
| External scanners | Current scope still named v0.53.0; scanner binding and `AcceptEOF` changes absent. | Name v0.54.0, keep Markdown Inline uncertified, and document language-bound symbols and end-token acceptance. [Scanner sources](https://github.com/odvcencio/gotreesitter/tree/v0.54.0/grammars/runtime), [lexer table API](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/language.go). |
| Migration | v0.50.0 availability warning mixed with promises about an unshipped shim. | State that v0.54.0 has no `compat/smacker`; give native API migration steps. [Tagged tree](https://github.com/odvcencio/gotreesitter/tree/v0.54.0). |
| Contributing | Stale job graph, manual-only exhaustive parity, and old hard performance thresholds. | Link the pinned workflow and describe the nightly sweep and admission-route check. [CI](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/.github/workflows/ci.yml), [roadmap](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/docs/roadmap.md). |
| Slide deck | 116 scanners, old canonical ratio, unverified adoption totals, and named customer examples. | Use 119 scanners and dated v9/board results; remove adoption totals and customer examples. PR #90 remains dated historical evidence with its final-median caveat. [Languages](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/docs/languages.md), [PR #90](https://github.com/odvcencio/gotreesitter/pull/90). |
| Production gate | Old evidence strings; playground used the docs budget profile. | Check v0.54.0 evidence and apply all four routes in `perf-budget.json`, including the playground network limit. |

The new `/docs/release-v0.54.0` page lists the complete grouped exported API additions,
including diagnostic counters, EOF receipt fields, scanner hooks, and build-tag stubs.
The tree-cursor page and playground guide need no release-specific API or count change.
The installed GoSX and TinyGo pins remain unchanged.

## Exclusions

Issues #1274 (Python list-splat) and #1275 (duplicate escape-sequence) are outside v0.54.0.
They are not documented as shipped. There is no new exact-tag parser benchmark or parity sweep
in this audit. The upstream changelog snapshots retain their original wording for source integrity.

## Gate repairs

The strict conference presentation gate required aspect ratio, caption space, duration,
offline metadata, and a fallback for the benchmark interaction. Those fields are now present.

The playground budget check exposed two existing delivery limits. The 24.3 MB parser WASM
was sent without compression. The builder now writes a Brotli transport variant and the
server negotiates it, with byte-equality, cache, HEAD, and fallback tests. The network ceiling
remains 5,200 KiB. The decoded JavaScript measurement was about 297 KiB for the pinned GoSX
runtime, engine chunk, Go shim, navigation, and analytics. The playground JS ceiling is now
320 KiB. The docs ceiling remains 160 KiB. No time or long-task ceiling was raised. CI now runs these browser budgets with the installed Chrome path.
Changelog sections also render introductory paragraphs and tables; these were previously hidden.
