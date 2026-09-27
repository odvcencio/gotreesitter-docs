---
title: Performance
description: Dated gotreesitter benchmark results, their scope, and v1 speed targets.
nav_group: Internals
order: 2
---

Version v0.55.1 uses production GLR parsing by default. Set
GTS_ADMISSION_CANDIDATE=1 to try the compact candidate route. Its per-language
allowlist is empty. Compact parser graduation remains in progress.

## v0.55.1 control on a quiet host

Upstream measured the exact v0.55.1 source, commit
[92db945f](https://github.com/odvcencio/gotreesitter/commit/92db945f28de67be51de8235c9cd4e25a900f648),
on 2026-09-27. The medians below are from gts-bench-1: C3-standard-8, Xeon
Platinum 8481C, four cores with SMT off, Ubuntu 24.04, and Go 1.26.4. The input
is a generated Go file with 500 functions that never forks. It is a control,
not a typical source file. The
[upstream BENCH.md receipt](https://github.com/odvcencio/gotreesitter/blob/main/BENCH.md#primary-trio-baseline)
and [PR #1355](https://github.com/odvcencio/gotreesitter/pull/1355) describe the measurement.

| Benchmark | Median | Allocations/op |
|---|---:|---:|
| Full parse | 8,686,195 ns/op | 8 |
| Single-byte edit | 177,281 ns/op | 5 |
| No-edit reparse | 8.318 ns/op | 0 |

The allocation counts come from the v0.55.1
[CHANGELOG measurement scope](https://github.com/odvcencio/gotreesitter/blob/v0.55.1/CHANGELOG.md#measurement-scope).
The host was pinned to CPU 2. Upstream used 20 shuffled seeds, one process per
seed, GOMAXPROCS=1, -benchtime=750ms, and -benchmem.

To run the same control on gotreesitter main, upstream used this command and
[benchmark script](https://github.com/odvcencio/gotreesitter/blob/main/scripts/bench_baseline.sh):

```sh
GOMAXPROCS=1 GOWORK=off taskset -c 2 bash scripts/bench_baseline.sh <out>
```

This script is on upstream main. It is not part of the v0.55.1 tag.

## Sealed Go-versus-C receipt

The hardware-attested v9 receipt measured four frozen Go files on 2026-08-02,
at build 492cd600, inside an AMD SEV Confidential Space VM. Its equal-fixture
geometric mean was 4.815x C for production and 3.986x C for compact. This is
not a v0.55.1 measurement. The signed receipt cannot be rerun outside
Confidential Space. Upstream keeps its scope and limits in the
[sealed receipt](https://github.com/odvcencio/gotreesitter/blob/main/BENCH.md#sealed-epoch--v9-hardware-attested-authoritative).

## v1.0 speed targets

The v1 design sets these targets. Owner decision O-Q3 makes the engine-floor
and tuned rows release-blocking for v1.0. Stretch targets remain directional.

| Layer | Scope | v1.0 target |
|---|---|---|
| Engine floor | All 206 grammars | Median at most 3x C; no file above 5x; no cliff failures. |
| Tuned | Top 50 grammars | Median at most 2.5x C; no language above 4x; 99th-percentile edit latency at most 20 ms at 137 KiB. |
| Stretch | Top 20 grammars | Full parse at most 2x C; edit latency at or below C. |

See [the v1 design](https://github.com/odvcencio/gotreesitter/blob/main/docs/v1-design.md#d13-engine-floor-tuned-and-stretch-targets)
for the complete gate and [The road to v1](/docs/v1) for the release plan.

## History

### v0.54.0 default-route comparison

The 2026-09-23 v0.54.0 release notes compare the prior default with the new
default on a fixed nine-language corpus of typical files. They name Buildbox's
tamarack harness but publish no hardware model. Lower Go/C ratios mean less
time. These results are historical and do not describe v0.55.1.

| Language | Prior default Go / C | v0.54.0 Go / C | Speedup |
|---|---:|---:|---:|
| Go | 5.97x | 5.42x | 1.10x |
| Python | 2.87x | 1.54x | 1.86x |
| TypeScript | 3.17x | 2.31x | 1.37x |
| Rust | 5.06x | 2.53x | 2.00x |
| YAML | 3.56x | 2.34x | 1.52x |
| Bash | 3.65x | 2.08x | 1.75x |
| Markdown | 6.36x | 2.87x | 2.22x |
| Lua | 4.66x | 2.33x | 2.00x |
| CSS | 4.60x | 2.36x | 1.95x |

TypeScript and YAML used the production fallback for every case in the prior
default: compact parsing declined, then production parsed again. The release
notes cite [PR #1264](https://github.com/odvcencio/gotreesitter/pull/1264).
They do not publish a combined ratio.

### v0.53.0 release comparison

The 2026-09-19 release notes compare v0.52.0 with the v0.53.0 candidate
48503fef. This historical run used twenty shuffled pairs, GOMAXPROCS=1,
-benchtime=750ms, the gts_parsercorephase0 tag, and one pinned Intel Core
Ultra 9 285 CPU in a container with 4 GiB of memory.

| Benchmark | v0.52.0 | v0.53.0 | Time change |
|---|---:|---:|---:|
| Full parse | 13.099 ms | 9.637 ms | -26.43% |
| Single-byte edit | 2,294.1 us | 127.5 us | -94.44% |
| No-edit reparse | 2.612 ns | 4.043 ns | +54.80% |

The v0.53.0 edited lane allocated five objects and its no-edit lane allocated
zero. These are historical results on one host, not v0.55.1 timings or C
comparisons. See the
[v0.54.0 release notes](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/CHANGELOG.md#0530---2026-09-19).

## Scope

Full parsing, edited parsing, no-edit reuse, and parser-core diagnostics are
separate workloads. The control above does not exercise GLR forks. Read each
receipt for its source revision, input, host, correctness checks, and timing
method. Report correctness separately from time and memory.

The project roadmap points to the current [v1 design](https://github.com/odvcencio/gotreesitter/blob/main/docs/v1-design.md).
See [Recovery and Correctness](/docs/recovery-and-correctness) for the published
correctness scopes.
