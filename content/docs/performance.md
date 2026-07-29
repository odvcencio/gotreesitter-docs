---
title: Performance
description: Canonical gotreesitter performance receipts, fleet ratios, memory results, and the methodology behind them.
nav_group: Internals
order: 2
---

gotreesitter measures interactive incremental work and cold full parsing separately. The
distinction matters: the pure-Go runtime runs exceptionally fast on editor-style reparses, while a
fresh materialized parse is currently slower than the C runtime on the canonical workload and
across much of the grammar fleet.

The repository's current [`BENCH.md`](https://github.com/odvcencio/gotreesitter/blob/v0.47.1/BENCH.md)
is the canonical source for linkable performance claims. This site runs on gotreesitter v0.47.1.
It follows the sealed v0.47.0 performance epoch. The public production `Parser.Parse` path has a
**5.526× C** equal-fixture geomean on the locked four-file matrix. The build-tagged compact path
measures **2.9975× C** on the same matrix. This compact path is diagnostic. It is not the public
parser. The site does not present it as production performance. v0.47.1 changes recovery
correctness only. It does not replace this receipt.

## Canonical full parse: the real-code matrix

The full-parse headline is measured on four immutable snapshots of clean, human-authored Go that
exercise genuine GLR forking (12–18 live stacks), against one fingerprinted static C oracle
(upstream tree-sitter v0.25.1, `-O2`, statically linked). The current complete publication receipt
uses a pinned quiet host, process-isolated samples per backend and fixture, and exact deep-tree
identity admitted before timing:

| Fixture | Production Go / C | Compact Go / C |
|---|---:|---:|
| `rewrite.go` (5.1 KB) | 5.042× | 3.077× |
| `query_compile.go` (20 KB) | 6.283× | 3.161× |
| `language.go` (41 KB) | 5.354× | 3.080× |
| `grammargen/lr.go` (236 KB) | 5.498× | 2.695× |
| **Equal-fixture geomean** | **5.526×** | **2.9975×** |

The sealed receipt uses the public `Parser.Parse` API for the production lane, a fingerprinted
tree-sitter v0.25.1 C oracle, at least ten seconds per fixture and backend pair, and A/A controls.
See `BENCH.md` for receipt identities and mandatory caveats.

The project withdrew an earlier **1.895× C** headline: it measured a generated 500-function Go
file that never forks (a straight-LR control, not representative code) against a C baseline built
from a different grammar. The control remains useful for tracking single-stack and incremental
fast paths:

| Lane (straight-LR control) | Result | Allocations |
|---|---:|---:|
| Full parse, materialized | 10.9 ms | 9 |
| Incremental, 1-byte edit | 1.98 µs | **0** |
| Incremental, no edit | 9.9 ns | **0** |

Absolute times are host-specific; the allocation counts are the portable claims.

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
return the old tree immediately. On the pinned receipt, both the one-byte edit and no-edit lanes
allocate nothing.

Earlier releases published incremental speedup multipliers against the cgo binding a Go
application would otherwise call, which pays a fixed per-call FFI cost that pure Go avoids. The
project withdrew those same-host calibration rows together with the old full-parse headline,
because the binding used a mismatched grammar; the portable claims today are the zero-allocation
fast paths above. Representative incremental timing on real code returns once the remaining
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
