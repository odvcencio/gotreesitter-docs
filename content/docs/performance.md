---
title: Performance
description: Canonical gotreesitter performance receipts, fleet ratios, memory results, and the methodology behind them.
nav_group: Internals
order: 2
---

gotreesitter measures edited incremental parsing, no-edit reuse, and fresh parsing separately.
Version 0.53.0 restores the same-width token-invariant shortcut behind authenticated lexical
dependency proofs. Ordinary subtree reuse and no-edit reuse remain available.

## v0.53.0 performance evidence

[Pull request #1093](https://github.com/odvcencio/gotreesitter/pull/1093) restores the shortcut
that v0.52.0 disabled. Complete lexical dependency proofs authenticate reuse before v0.53.0
applies it. The [v0.52.0 mitigation](#v0520-mitigation-historical) below no longer describes
this release.

Paired randomized benchmarks compare v0.52.0 with the v0.53.0 candidate code. The run used 20
shuffle seeds, alternating order, `-benchtime=750ms`, `GOMAXPROCS=1`, and one pinned CPU (Intel
Core Ultra 9 285). All three time changes have p=0.000 with n=20.

| Benchmark | v0.52.0 | v0.53.0 | Change |
| --- | ---: | ---: | ---: |
| `BenchmarkGoParseFullDFA` | 13.099 ms | 9.637 ms | -26.43% |
| `BenchmarkGoParseIncrementalSingleByteEditDFA` | 2,294.1 µs | 127.5 µs | -94.44% |
| `BenchmarkGoParseIncrementalNoEditDFA` | 2.612 ns | 4.043 ns | +54.80% |

The single-byte edit gain comes from the restored token-invariant reuse. The no-edit path grew
by 1.4 ns because the unchanged-tree fast path now adds a tree handle and compares included
ranges; it still allocates zero bytes. Each returned tree now allocates one new `Tree` value,
because released trees no longer return to a pool.

A one-shot run on the `grammargen/lr.go` large-file fixture recorded maximum resident set size
under `/usr/bin/time -v`, `GOMAXPROCS=1`, `-benchtime=1x`:

| Measure | v0.52.0 | v0.53.0 |
| --- | ---: | ---: |
| Maximum resident set size | 151,444 KiB | 137,540 KiB |
| Bytes allocated for each parse | 4,304,096 B | 134,560 B |
| Allocations for each parse | 64,132 | 3,080 |

This comparison measures two Go releases on one pinned host. It does not time C and does not
establish compact parser graduation, which remains unfinished. See
[`BENCH.md`](https://github.com/odvcencio/gotreesitter/blob/main/BENCH.md) for the canonical
C-ratio claims.

## v0.52.0 mitigation (historical)

Version 0.53.0 restored the shortcut this section describes as disabled. The numbers below
remain the sealed v0.52.0 receipt.

The measured single-byte edit rose from 1.706 microseconds to 3,350.460 microseconds,
approximately **1,964 times slower** than the baseline. It allocated 184.4 KiB
and 95 objects per operation instead of zero.
The owner approved disabling the unsafe shortcut for that release. Restoring it required complete
lexical dependency proofs under [issue #1087](https://github.com/odvcencio/gotreesitter/issues/1087).

Full-parse timing changed from 19.79 ms to 19.52 ms, without statistical significance.
Full parsing reduced allocated bytes by **42.90%** and allocation count by **99.72%**.
No-edit timing changed from 3.576 ns to 3.619 ns, without statistical significance.
No-edit parsing retained zero allocations.

The comparison used twenty alternating same-seed pairs on a shared WSL host. It measured both
release optimizations and the mitigation together, and it did not time C or establish compact
parser graduation.

## Historical receipts

The repository's current [`BENCH.md`](https://github.com/odvcencio/gotreesitter/blob/main/BENCH.md)
is the canonical source for linkable performance claims. The sealed v0.45.0 epoch contains two
route receipts on the locked four-file matrix: **5.526× C** for production parsing and **2.9975×
C** for compact parsing.

Version 0.48 changes fresh full-parse dispatch. It attempts compact parsing only for an eligible
fresh full parse. When compact parsing declines, production parsing returns the tree. This
fail-closed dispatch preserves the production result. Do not combine the two sealed route
receipts into a v0.48.0 full-parser headline.

## Canonical full parse: the real-code matrix

The full-parse headline is measured on four immutable snapshots of clean, human-authored Go that
exercise genuine GLR forking (12–18 live stacks), against one fingerprinted static C oracle
(upstream tree-sitter v0.25.1, `-O2`, statically linked). The current complete publication receipt
uses a pinned quiet host, process-isolated samples per backend and fixture, and exact deep-tree
identity admitted before timing:

| Fixture | Sealed v0.45.0 production Go / C | Sealed v0.45.0 compact Go / C |
|---|---:|---:|
| `rewrite.go` (5.1 KB) | 5.042× | 3.077× |
| `query_compile.go` (20 KB) | 6.283× | 3.161× |
| `language.go` (41 KB) | 5.354× | 3.080× |
| `grammargen/lr.go` (236 KB) | 5.498× | 2.695× |
| **Equal-fixture geomean** | **5.526×** | **2.9975×** |

The sealed receipt uses the public `Parser.Parse` API for the production lane, a fingerprinted
tree-sitter v0.25.1 C oracle, at least ten seconds per fixture and backend pair, and A/A controls.
It measures each route independently. See `BENCH.md` for receipt identities and mandatory
caveats.

The v0.48 bounded real-corpus matrix contains 110 selected rows: 70 compact passes, 30 production
fallbacks, and 10 skips. It contains no divergence or error. The compact route serves eligible
fresh full parses only. Production parsing remains the fallback for every declined or ineligible
input.

The project withdrew an earlier **1.895× C** headline: it measured a generated 500-function Go
file that never forks (a straight-LR control, not representative code) against a C baseline built
from a different grammar. The control remains useful for tracking single-stack and incremental
fast paths:

| Lane (straight-LR control) | Result | Allocations |
|---|---:|---:|
| Full parse, materialized | 10.9 ms | 9 |
| Incremental, 1-byte edit | 1.98 µs | **0** |
| Incremental, no edit | 9.9 ns | **0** |

These historical allocation counts do not describe edited parsing in v0.53.0, which allocates 5
objects for the restored single-byte edit lane.
Absolute times are host-specific.

```sh
GOMAXPROCS=1 go test . -run '^$' \
  -bench 'BenchmarkGoParseFullDFA|BenchmarkGoParseIncrementalSingleByteEditDFA|BenchmarkGoParseIncrementalNoEditDFA' \
  -benchmem -count=10 -benchtime=750ms
```

> [!IMPORTANT] Benchmark integrity correction
> Before v0.24.1, `BenchmarkGoParseFullDFA` silently selected a no-tree diagnostic path. The old
> 1.54 ms, 728 B/op, and 7 allocs/op headline therefore did **not** describe a materialized public
> parse, and the project withdrew it. `BenchmarkGoParseCoreDFA` remains useful for attribution,
> but its results are never presented as full-parse performance.

## Why incremental work is different

An edit invalidates a narrow span. `ParseIncremental` can reuse unchanged subtrees, their parser
states, and external-scanner checkpoints instead of rebuilding the document. A no-edit call can
return the old tree immediately. The historical receipt reports zero allocations
for both incremental lanes. Version 0.53.0 preserves that result only for the
measured no-edit control; the restored single-byte edit lane allocates 5 objects,
far fewer than the v0.52.0 mitigation's 95.

Earlier releases published incremental speedup multipliers against the cgo binding a Go
application would otherwise call, which pays a fixed per-call FFI cost that pure Go avoids. The
project withdrew those same-host calibration rows together with the old full-parse headline,
because the binding used a mismatched grammar. Do not apply those historical
zero-allocation edit claims to v0.53.0. Representative incremental timing on real code returns once the remaining
incremental/fresh tree-identity work closes — correctness gates timing here.

## Full parse across the grammar fleet

Full-parse behavior is a distribution, not one marketing number. The ratcheted real-corpus ledger
covers 204 of 206 grammars; D and F# are named held-outs. As of the 2026-07-11 ledger:

| Go time / C time | Languages |
|---|---:|
| At or faster than C | 10 |
| 1–2× C | 64 |
| 2–3× C | 29 |
| More than 3× C | 101 |

The observed median is about **3× C**. Many high ratios come from small DSL files, where C
finishes in microseconds and fixed per-parse work dominates. Others reflect real ambiguity,
recovery, or memory cliffs. The scoreboard keeps those classes visible rather than averaging them
away.

Named large-file witnesses include JavaScript's 3.4 MB Poppler file, TypeScript's generated
`webworker.generated.d.ts`, Groovy's `pleac11_15.groovy`, and generated Go tables. Poppler reaches
exact structural parity inside a hard 2 GiB container, but its full parse remains 3.50× C, and its
ordinary 512 MiB budget path does not yet complete economically.

## Memory receipts

The v0.24–v0.26.1 memory campaign materially changed the retained-tree cost:

- The Go node header fell from 144 to **104 bytes** through arena-backed field sidecars.
- Final-tree compaction cut Poppler's retained post-GC heap from 862,803,056 to **409,862,040
  bytes**, a 52.5% reduction.
- Bounded raw-shape reclamation removed another **192 MiB** of retained data on the witness.

Those results measure retained memory; they do not claim that peak RSS or full-parse latency
beats C. Every accepted memory change preserved the exact selected C-oracle tree.

## How results are gated

Correctness and performance are separate gates:

1. A change first preserves a complete, byte-exact selected tree against the pinned C oracle.
2. The same workload is measured before and after with stable settings.
3. `benchstat` must improve the targeted metric without regressing the canonical trio.
4. Large-file work records maximum RSS as well as `ns/op`, `B/op`, and `allocs/op`.
5. Fleet budgets ratchet tighter; caveats, timeouts, held-outs, and stopped parses remain named.

Quiet, reproducible, one-language runs move the ratchets. Measurements from a contended host
serve as useful smoke evidence, not release-grade performance claims.

> [!IMPORTANT] Read with correctness
> Performance cannot establish that the selected tree is right. See [Recovery and
> Correctness](/docs/recovery-and-correctness) for the oracle and real-corpus parity gates.
