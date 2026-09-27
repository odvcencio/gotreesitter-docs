---
title: v0.55 Releases
description: What changed in gotreesitter v0.55.0 and v0.55.1, with pull requests, measurement scope, and known gaps.
nav_group: Project
order: 3
---

This page summarizes gotreesitter v0.55.0 (2026-09-24) and v0.55.1 (2026-09-26).
The tagged [CHANGELOG](https://github.com/odvcencio/gotreesitter/blob/v0.55.1/CHANGELOG.md)
is the source for release entries and their linked evidence.

## v0.55.1 — 2026-09-26

### Added

The release changelog lists no added code features.

### Changed

The release changelog lists no behavior changes.

### Fixed

- [PR #1290](https://github.com/odvcencio/gotreesitter/pull/1290) adjusts Python's initial
  stack cap when an input cannot contain an unpacking expression, while preserving a wider cap
  when unpacking is possible and keeping explicit overrides.
- [PR #1291](https://github.com/odvcencio/gotreesitter/pull/1291) preserves conflict forks beyond
  the earlier depth limit and resets the dispatch guard when reductions reach a new minimum depth.
- [PR #1292](https://github.com/odvcencio/gotreesitter/pull/1292) removes a duplicate clone during
  hidden-node alias materialization and extends the low-yield reuse guard to larger roots.

### Documentation

- [PR #1329](https://github.com/odvcencio/gotreesitter/pull/1329) aligns the roadmap, repository
  map, and agent workflow with the owner's v1 design. It changes no public API or admission default.

### Measurement scope

The v0.55.1 changelog reports primary benchmark comparisons with twenty shuffled seeds,
750 ms per case, one process per seed, and `GOMAXPROCS=1`. It reports no significant change in
primary timing and allocations of eight, five, and zero per operation. These Linux measurements
do not establish Windows performance. The linked pull requests carry separate fixture results;
their hosts and source revisions do not represent one shared release run. The allocation counts
are also present in the exact-tag 2026-09-27 control on gts-bench-1. For its source, host details,
command, and generated non-forking input, see [Performance](/docs/performance).

### Known gaps

The v0.55.1 changelog lists unresolved issues: transient-error incremental trees, `ERROR` roots
with false `HasError()`, and diff and LESS edit mismatches remain open and excluded. C# still misses
its sub-second target. Scala retains extra work from the refreshed grammar and suffix gaps. VHDL
inner `@spell` captures, blank HTTP comments, and compact sibling traversal remain unresolved.
Django tree differences, CSV comma witnesses, and the PHP compact recovery winner remain open.

### Field report

The release notes say this patch addressed the Python regression and false C errors reported in
[issue #454](https://github.com/odvcencio/gotreesitter/issues/454) by
[@cognociente](https://github.com/cognociente).

## v0.55.0 — 2026-09-24

### Added

- [PR #1281](https://github.com/odvcencio/gotreesitter/pull/1281) adds a highlighter option to
  select the parser route for documents and injected parsers.
- [PR #1285](https://github.com/odvcencio/gotreesitter/pull/1285) adds optional compact-parser
  telemetry for peak frontier headers and derivations.

### Changed

- [PR #1281](https://github.com/odvcencio/gotreesitter/pull/1281) gives explicit process settings
  priority over the language allowlist. A parser override remains highest priority; the allowlist
  widens only the implicit default. `SetAdmissionCandidateRouteDefault(false)` and
  `GTS_ADMISSION_CANDIDATE=0` keep production selected.

### Fixed

- [PR #1284](https://github.com/odvcencio/gotreesitter/pull/1284) ignores `Accept` when compact
  reduction checks whether a source can shift.
- [PR #1286](https://github.com/odvcencio/gotreesitter/pull/1286) elects accepted material paths
  inside the winning compact recovery group and declines unsupported paths without publishing a
  guessed tree.
- [PR #1287](https://github.com/odvcencio/gotreesitter/pull/1287) bounds end-of-file recovery
  versions before allocating reduction parents.
- [PR #1276](https://github.com/odvcencio/gotreesitter/pull/1276) fixes overlapping Python
  `escape_sequence` nodes after escaped backslashes and fixes
  [issue #1275](https://github.com/odvcencio/gotreesitter/issues/1275).
- [PR #1277](https://github.com/odvcencio/gotreesitter/pull/1277) preserves Python `list_splat`
  binding for attribute and subscript suffixes and fixes
  [issue #1274](https://github.com/odvcencio/gotreesitter/issues/1274).
- [PR #1278](https://github.com/odvcencio/gotreesitter/pull/1278) rejects query runs that cannot
  reach a required successor before enumerating captures, preserves highlight capture order, and
  reports quantified roots as non-rooted in `IsPatternRooted`.
- [PR #1279](https://github.com/odvcencio/gotreesitter/pull/1279) caches raw-shape error costs
  during GLR elections, bounds graph reachability checks, and rejects impossible merges earlier.
- [PR #1282](https://github.com/odvcencio/gotreesitter/pull/1282) limits Make forest rescue,
  builds eligible C-certified HTTP sections directly, and retries a fresh parse when incremental
  reuse makes too little progress.

### Measurement scope

The v0.55.0 changelog describes Linux amd64 results and says they do not establish Windows
performance. Make, HTTP, and Dart used deterministic reconstructions; C# used the reporter's
fixture and its host load varied. The Nushell baseline used one repetition and the after result
used the median of three. The linked pull requests contain the individual fixtures, hosts, source
revisions, and commands. Their results are separate measurements; do not combine them into one
release-wide timing claim.

### Known gaps

The v0.55.0 changelog lists open items that include transient-error incremental trees, diff and
LESS edit mismatches, and locked-C failures for malformed JavaScript, LESS, and TOML. C# remains
above its sub-second target, Scala's regression remains unresolved, blank HTTP comments still need
a locked-C regression, Django tree differences remain, CSV comma inputs still differ from locked C,
and the PHP compact recovery winner remains open.

See the tagged [v0.55.0 and v0.55.1 CHANGELOG entries](https://github.com/odvcencio/gotreesitter/blob/v0.55.1/CHANGELOG.md)
for original results and links to their evidence.
