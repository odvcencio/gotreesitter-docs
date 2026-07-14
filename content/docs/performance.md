---
title: Performance
description: Canonical gotreesitter performance receipts, fleet ratios, memory results, and the methodology behind them.
nav_group: Internals
order: 2
---

gotreesitter measures interactive incremental work and cold full parsing separately. The
distinction matters: the pure-Go runtime is exceptionally fast on editor-style reparses, while a
fresh materialized parse is currently slower than the C runtime on the canonical workload and
across much of the grammar fleet.

The repository's [`BENCH.md`](https://github.com/odvcencio/gotreesitter/blob/v0.36.0/BENCH.md)
is the canonical source for linkable performance claims. This page follows the receipts published
with v0.36.0.

## Corrected canonical benchmark

The public benchmark uses a generated 500-function Go file (19,294 bytes), calls the ordinary
`Parser.Parse` API, requires a complete root, materializes the returned tree, and releases it.
Stable runs use one pinned core, `GOMAXPROCS=1`, ten samples, a 750 ms benchtime, and allocation
reporting.

| Lane | Pure Go | C through cgo | Go / C | Go allocations |
|---|---:|---:|---:|---:|
| Full parse, materialized | 10.907 ms | 5.756 ms | **1.895×** | 9 |
| Incremental, 1-byte edit | 1.98 µs | 331 µs | **0.006× — 167× faster** | 0 |
| Incremental, no edit | 9.9 ns | 330 µs | **about 33,000× faster** | 0 |

These are the canonical receipts published with v0.36.0 from one pinned host and workload
contract. Each row is independently release-pinned rather than produced by a single benchmark
run. Absolute times are host-specific; the same-host ratios and allocation counts are the
portable claims.

```sh
GOMAXPROCS=1 go test . -run '^$' \
  -bench 'BenchmarkGoParseFullDFA|BenchmarkGoParseIncrementalSingleByteEditDFA|BenchmarkGoParseIncrementalNoEditDFA' \
  -benchmem -count=10 -benchtime=750ms
```

> [!IMPORTANT] Benchmark integrity correction
> Before v0.24.1, `BenchmarkGoParseFullDFA` silently selected a no-tree diagnostic path. That
> pre-correction headline therefore did **not** describe a materialized public parse and has been
> withdrawn. `BenchmarkGoParseCoreDFA` remains useful for attribution, but its results are never
> presented as full-parse performance.

## Why incremental work is different

An edit invalidates a narrow span. `ParseIncremental` can reuse unchanged subtrees, their parser
states, and external-scanner checkpoints instead of rebuilding the document. A no-edit call can
return the old tree immediately. On the pinned receipt, both the one-byte edit and no-edit lanes
allocate nothing.

The comparison is against the cgo binding a Go application would actually call. That path pays a
roughly 330 µs fixed per-call cost on this workload, while gotreesitter stays inside the Go runtime.
This is why the incremental advantage is much larger than the full-parse comparison.

## Full parse across the grammar fleet

Full-parse behavior is a distribution, not one marketing number. The ratcheted real-corpus ledger
covers 204 of 206 grammars; D and F# are named held-outs. As of the 2026-07-11 ledger:

| Go time / C time | Languages |
|---|---:|
| At or faster than C | 10 |
| 1–2× C | 64 |
| 2–3× C | 29 |
| More than 3× C | 101 |

The observed median is about **3× C**. Many high ratios come from small DSL files where C finishes
in microseconds and fixed per-parse work dominates. Others are real ambiguity, recovery, or memory
cliffs. The scoreboard keeps those classes visible rather than averaging them away.

Named large-file witnesses include JavaScript's 3.4 MB Poppler file, TypeScript's generated
`webworker.generated.d.ts`, Groovy's `pleac11_15.groovy`, and generated Go tables. Poppler reaches
exact structural parity inside a hard 2 GiB container, but its full parse remains 3.50× C and its
ordinary 512 MiB budget path does not yet complete economically.

## Memory receipts

The v0.24–v0.26.1 memory campaign materially changed the retained-tree cost:

- The Go node header fell from 144 to **104 bytes** through arena-backed field sidecars.
- Final-tree compaction cut Poppler's retained post-GC heap from 862,803,056 to
  **409,862,040 bytes**, a 52.5% reduction.
- Bounded raw-shape reclamation removed another **192 MiB** of retained data on the witness.

Those are retained-memory results, not a claim that peak RSS or full-parse latency beats C. Every
accepted memory change preserved the exact selected C-oracle tree.

## How results are gated

Correctness and performance are separate gates:

1. A change first preserves a complete, byte-exact selected tree against the pinned C oracle.
2. The same workload is measured before and after with stable settings.
3. `benchstat` must improve the targeted metric without regressing the canonical trio.
4. Large-file work records maximum RSS as well as `ns/op`, `B/op`, and `allocs/op`.
5. Fleet budgets ratchet tighter; caveats, timeouts, held-outs, and stopped parses remain named.

Quiet, reproducible, one-language runs move the ratchets. Measurements from a contended host are
useful smoke evidence, not release-grade performance claims.

> [!IMPORTANT] Read with correctness
> Performance cannot establish that the selected tree is right. See [Recovery and
> Correctness](/docs/recovery-and-correctness) for the oracle and real-corpus parity gates.
