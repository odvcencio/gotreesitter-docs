---
title: Contributing
description: Dev setup, the test suite, the C-oracle parity harness, the perf-scan harness, and the gates a PR has to pass.
nav_group: Project
order: 2
---

## Dev setup

The root module (`github.com/odvcencio/gotreesitter`) is pure Go: no cgo, no C toolchain. `go.mod`
pins a minimum of **Go 1.22**. The parity/oracle work lives in a second module, `cgo_harness`,
kept separate specifically so the root module's own dependency metadata stays cgo-free. Its
`go.mod` requires **Go 1.25** and links the real C tree-sitter runtime, so building it needs a C
toolchain. `cgo_harness/go.mod` points back at your checkout with
`replace github.com/odvcencio/gotreesitter => ..`, so a harness run always tests your local
changes.

This repo's own docs prefix `go run`/`go test` commands with `GOWORK=off`. Do the same, especially
when you move between the root module and `cgo_harness` — it keeps module resolution pinned to
the `replace` directives here instead of drifting if you have an ambient Go workspace elsewhere.

## Running the tests

The fast local loop mirrors the start of CI's `build` job:

```sh
go build ./...
go vet ./...
go test ./... -run '^$' -count=1   # compiles every test binary; runs nothing
```

For anything beyond that, narrow with `-run`: `go test . -run '^TestName$' -count=1 -v`. Do
**not** run `go test ./...` or `go test ./... -race` broadly on your own machine — both the
README's Testing section and `AGENTS.md` call this out explicitly, since a wide host-side sweep
can exhaust memory on a dev machine. Use CI, or the Docker runners below, for anything wider than
a focused package run. `go run ./cmd/parity_report` gives a quick smoke-correctness status across
all 206 grammars.

## The oracle: C parity harness

`cgo_harness` exists to answer one question: does gotreesitter produce the same tree as the real
C tree-sitter runtime? That differential comparison — "use differential testing against the C
runtime as the primary oracle," in the harness's own framework doc — is the project's core
correctness discipline. For error recovery specifically, the check goes deeper than tree diffing:
gotreesitter's recovery port is verified decision-by-decision against `parser.c`'s own recovery
functions (see [Architecture](/docs/architecture)), not just compared on final output.

Parity tests live behind the `treesitter_c_parity` build tag. For local work, run one grammar at a
time in an isolated container rather than a broad sweep:

```sh
bash cgo_harness/docker/run_single_grammar_parity.sh typescript
```

The direct form still targets CI/lab use, not routine host sweeps:

```sh
cd cgo_harness
go test . -tags treesitter_c_parity \
  -run '^TestParityFreshParse$|^TestParityHasNoErrors$|^TestParityIssue3Repros$' \
  -count=1 -v
```

`GTS_PARITY_MODE=smoke|top50|exhaustive` scales the corpus — `smoke` is the small, CI-sized
default. The grammargen compiler currently targets eight high-value languages for its own
validation (css, javascript, typescript, tsx, c, c_sharp, cobol, fortran): it checks
grammargen-generated tables against real corpus and against the C oracle. Use the focused runner
for that, one language per container:

```sh
bash cgo_harness/docker/run_grammargen_focus_targets.sh --mode real-corpus --langs typescript
bash cgo_harness/docker/run_grammargen_focus_targets.sh --mode cgo --langs typescript
```

## The perf-scan harness

`cgo_harness/zz_perf_scan_test.go` (tags `treesitter_c_parity treesitter_c_perfscan`, gated by
`GTS_PERF_SCAN=1`) times gotreesitter against C tree-sitter per real-corpus file, per axis
(`full`, `noedit`, and an opt-in `edit` axis). It buckets every language into a verdict — `<=1.2x`,
`<=2x`, `>2x`, or `cliff>10x` — so a single pathological file cannot hide behind a healthy average:

```sh
cd cgo_harness
GOWORK=off GTS_PARITY_ALLOW_HOST=1 GTS_PERF_SCAN=1 \
  GTS_PERF_SCAN_LANGS=go,python,bash \
  go test -tags "treesitter_c_parity treesitter_c_perfscan" \
  -run '^TestPerfScanSweep$' -v -count=1 -timeout 0 .
```

Results land in `perf_scan/out/<run>/scoreboard.{json,md}` — see [Performance](/docs/performance)
for what the verdict buckets and budgets mean. Day-to-day micro-benchmarks live in the root
package instead: `BenchmarkGoParseFullDFA`, `BenchmarkGoParseIncrementalSingleByteEditDFA`, and
`BenchmarkGoParseIncrementalNoEditDFA` are the trio CI's perf gate tracks, run with stable
settings (`GOMAXPROCS=1 -count=10 -benchtime=750ms -benchmem`).

## Adding or updating a grammar

To add one of the 206 grammars embedded in this repo:

1. Add the grammar's repo to `grammars/languages.manifest`.
2. Refresh pinned refs: `go run ./cmd/grammar_updater -lock grammars/languages.lock -write -report grammars/grammar_updates.json`.
3. Generate tables: `go run ./cmd/ts2go -manifest grammars/languages.manifest -outdir ./grammars -package grammars -compact=true`.
4. Add smoke samples to `cmd/parity_report/main.go` and `grammars/parse_support_test.go`.
5. Verify: `go run ./cmd/parity_report && go test ./grammars/...`.

If the update touches `src/scanner.c` or the grammar's external-token list, treat it as scanner
work and update the matching hand-written Go scanner before regenerating blobs — see
[External Scanners](/docs/external-scanners). A JSON-only change with unchanged externals does
not need that extra step; regenerate the blob the normal way for however that grammar is built
(`ts2go` for most of the 206, `grammargen` for a growing subset).

To add a language from your own module without forking gotreesitter at all — the more common case
for a downstream consumer — see [Authoring Languages](/docs/authoring-languages) instead. The
path there is `grammargen.ImportGrammarJSON` → `grammargen.GenerateLanguageAndBlob` →
`grammars.Register`/`RegisterExtension`.

## Correctness gates a PR must pass

CI (`.github/workflows/ci.yml`) runs a `build` job on every push and PR:

- `go build ./...` and `go vet ./...`.
- `go test ./... -run '^$' -count=1` — compiles every test binary without running any of them, so
  the run catches a test file that fails to compile before the expensive `-race` run below.
- `GOTREESITTER_GRAMMAR_BLOB_DIR=grammars/grammar_blobs go run -tags grammar_blobs_external ./cmd/parity_report`.

Once a PR leaves draft status, the same job also runs the full race suite —
`go test $(go list ./... | grep -v '/grammargen$') -race -count=1 -timeout 35m -p 1` — plus a
non-blocking `go test ./grammargen -race` visibility run (`grammargen` carries a known
pre-existing backlog and does not block yet). While a PR is still a draft, it instead gets a
small hand-picked focused test list, plus a separate `draft-correctness` job that runs
`go test . ./grammars -count=1 -timeout 25m` (no `-race`) on every push. This keeps regressions
visible before the PR is marked ready, rather than only once the full suite finally runs.

Three more jobs gate every PR:

- **`freshness`** regenerates `grammars/linguist_gen.go` and fails if it changed. It also checks
  that `grammars/languages.lock`, `grammars/grammar_blobs/*.bin`, and `embedded_grammars_gen.go`
  all agree on the same set of grammars.
- **`parity-cgo`** runs a `GTS_PARITY_MODE=smoke` C-oracle gate in Docker on every PR and on
  `main`. The full exhaustive sweep (`parity-cgo-exhaustive`) runs only on manual
  `workflow_dispatch`.
- **`perf-regression`** benchmarks the PR against its base with `cmd/benchgate` (max +8% ns/op,
  +5% B/op, +5% allocs/op, +10% RSS on the bench trio above, plus an RSS check on a large
  full-parse run). It is a non-blocking warning while the PR is a draft, and a hard gate once the
  PR is marked ready for review.

`AGENTS.md` at the repo root has the fuller day-to-day workflow these gates assume — Docker
isolation, one language at a time, correctness before performance.
