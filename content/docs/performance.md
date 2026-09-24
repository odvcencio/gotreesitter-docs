---
title: Performance
description: Dated gotreesitter benchmark results, their scope, and the v0.54.0 default route.
nav_group: Internals
order: 2
---

Version v0.54.0 uses the production GLR route by default. Set
`GTS_ADMISSION_CANDIDATE=1` to enable the compact candidate route.
Compact parser graduation remains incomplete. The measurements below come from the
[tagged BENCH.md](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/BENCH.md)
and [release notes](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/CHANGELOG.md#0540---2026-09-23).
Each result applies to its stated revision, host, and workload.

## v0.54.0 default-route comparison

The release notes dated 2026-09-23 report three independent, interleaved runs.
The harness compares Go and C in one process on a fixed nine-language corpus of typical files.
The table compares the prior default with the release candidate. Lower Go/C ratios mean less time.
These ratios are not results for the four-file Go matrix below.

| Language | Prior default Go / C | New default Go / C | Speedup |
|---|---:|---:|---:|
| Go | 5.97× | 5.42× | 1.10× |
| Python | 2.87× | 1.54× | 1.86× |
| TypeScript | 3.17× | 2.31× | 1.37× |
| Rust | 5.06× | 2.53× | 2.00× |
| YAML | 3.56× | 2.34× | 1.52× |
| Bash | 3.65× | 2.08× | 1.75× |
| Markdown | 6.36× | 2.87× | 2.22× |
| Lua | 4.66× | 2.33× | 2.00× |
| CSS | 4.60× | 2.36× | 1.95× |

TypeScript and YAML had 100% fallback under the prior default: compact parsing declined,
then production parsing repeated the work. The release notes cite
[PR #1264](https://github.com/odvcencio/gotreesitter/pull/1264).
They do not publish a combined ratio. Do not infer one for other inputs.

## Quiet-host control: 2026-09-23

The dated BENCH.md receipt uses revision `4637be52a`, an Intel Xeon D-2141I at 2.20 GHz,
`GOMAXPROCS=1`, and medians of ten runs with `-benchtime=750ms`.
The input is a generated 500-function Go file. It is a single-stack control, not representative real code.

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkGoParseFullDFA` | 61,938,000 | 675,394 | 45 |
| `BenchmarkGoParseIncrementalSingleByteEditDFA` | 439,825 | 410 | 5 |
| `BenchmarkGoParseIncrementalNoEditDFA` | 27.4 | 0 | 0 |

The single-byte edit takes 439.825 µs and allocates five objects. Only the no-edit lane
allocates zero. This receipt is dated and pinned; it is not a measurement of the exact
v0.54.0 tag. Do not combine it with the v0.53.0 results from another host.

```sh
GOWORK=off GOMAXPROCS=1 go test . -run '^$' \
  -bench 'BenchmarkGoParseFullDFA|BenchmarkGoParseIncrementalSingleByteEditDFA|BenchmarkGoParseIncrementalNoEditDFA' \
  -benchmem -count=10 -benchtime=750ms
```

## Real-code matrix: sealed v9 receipt

BENCH.md identifies run `strictboundary-20260802T062212Z-v9` (2026-08-02,
build commit `492cd600`) as its current sealed Go/C evidence. It measures four frozen Go files.
It does not measure the v0.54.0 default dispatcher.

| Fixture | Production Go / C | Compact Go / C |
|---|---:|---:|
| `rewrite.go` | 4.653× | 4.348× |
| `query_compile.go` | 4.509× | 4.030× |
| `language.go` | 4.223× | 4.126× |
| `grammargen/lr.go` | 6.065× | 3.493× |
| Equal-fixture geometric mean | 4.815× | 3.986× |

Each Go iteration calls public `Parser.Parse` and checks root completeness and errors.
The C lane makes the same checks. Neither timed lane walks the full tree.
The run uses the locked tree-sitter v0.25.1 C oracle, `GOMAXPROCS=1`, at least ten seconds
per fixture and backend, `GOAMD64=v3`, and a pinned profile for Go profile-guided optimization.
The artifact includes hardware attestation and a signed receipt.

The production self-comparison has a geometric mean of 0.9989 and a maximum absolute delta
of 1.42%. The C self-comparison has a geometric mean of 0.9985 and a maximum absolute delta
of 0.69%. These are reported measurements, not pass/fail thresholds.

The source states these limits:

- The run used the ten-second floor only. It did not resample at 750 ms or five seconds.
- The C binary is identical to the v8, run6, and v0.45.0 oracle.
- The Go build differs from v8 by 46 commits. The receipt cannot isolate one change or separate
  the combined changes from hardware variation between virtual machines.
- No target ratio was set for this run. The results are not adjusted to a target.

The older v0.45.0 values, 5.526× C for production and 2.9975× C for compact, are historical.
They are not current full-parser claims. See the
[sealed receipt and caveats](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/BENCH.md#sealed-epoch--v9-hardware-attested-authoritative).

## Historical v0.53.0 release comparison

The [2026-09-19 release notes](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/CHANGELOG.md#0530---2026-09-19)
compare v0.52.0 with v0.53.0 candidate `48503fef`. The run uses twenty shuffled pairs,
`GOMAXPROCS=1`, `-benchtime=750ms`, the `gts_parsercorephase0` tag, and one pinned
Intel Core Ultra 9 285 CPU in a container with 4 GiB of memory.

| Benchmark | v0.52.0 | v0.53.0 | Time change |
|---|---:|---:|---:|
| Full parse | 13.099 ms | 9.637 ms | -26.43% |
| Single-byte edit | 2,294.1 µs | 127.5 µs | -94.44% |
| No-edit reparse | 2.612 ns | 4.043 ns | +54.80% |

The edited lane allocated five objects in v0.53.0. The no-edit lane allocated zero.
These are historical comparisons on one host, not v0.54.0 timings or C comparisons.

## Scope and reproduction

Full parsing, edited parsing, no-edit reuse, and parser-core diagnostics are separate workloads.
The withdrawn 1.54 ms diagnostic omitted tree materialization. Do not use it as full-parse evidence.
The withdrawn 1.895× C comparison also used different grammar artifacts.

Use the fixture hashes, compiler flags, oracle hashes, and commands in the tagged BENCH.md
when you reproduce a measurement. Report correctness separately from time and memory.
The [roadmap](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/docs/roadmap.md)
sets public `Parser.Parse` at no more than 1.5× C as a future target. It is not an achieved result.
See [Recovery and Correctness](/docs/recovery-and-correctness) for the dated parity scope.
