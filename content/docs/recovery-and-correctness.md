---
title: Recovery and Correctness
description: What the published C-oracle comparisons cover for parse trees and recovery, and where known differences remain.
nav_group: Internals
order: 1
---

The published recovery board records agreement on 39 of 79 cases, measured on 2026-09-19.
Results are specific to each test set: a match on one fixture does not establish parity for every
input. This page explains how the project compares Go trees with the pinned C tree-sitter v0.25.1
oracle and summarizes the published scope.

## What error recovery is

Real code is frequently incomplete or invalid. An editor reparses on almost every keystroke, so
the parser spends much of its life looking at a half-typed function, an unbalanced brace, or a
paste that dropped a line. A parser that only accepted grammatically perfect input would be
useless for tooling.

When the input does not fit the grammar, tree-sitter can recover and return a tree marked with
two special node kinds:

- **`ERROR`** — a span the parser could not fit into any grammar rule. Its children are the
  tokens it salvaged inside that span.
- **`MISSING`** — a zero-width node the parser *inserted* to complete a rule, when the grammar
  says a token should be there and it is not.

You can see both directly. Parse an unterminated JSON array:

```go
lang := grammars.JsonLanguage()
parser := gts.NewParser(lang)

tree, _ := parser.Parse([]byte(`[1, 2`))
fmt.Println(tree.RootNode().HasError()) // true
```

This is the recovery tree gotreesitter returns for `[1, 2`:

```text
document [0-5]
  array [0-5]
    [       [0-1]
    number  [1-2]
    ,       [2-3]
    number  [4-5]
    ]       MISSING [5-5]
```

The parser inserted a zero-width `]` at byte 5 so the `array` rule could close. That is a
*recovery decision*: which token to insert, where to resynchronize, and how much to wrap in
`ERROR`. The published 39-of-79 board records the tested agreement with the C oracle; this single
example describes gotreesitter output and does not expand that board.

At the API level, recovery surfaces through the node predicates: `HasError()` (this subtree
contains damage), `IsError()` (this node *is* an `ERROR`), and `IsMissing()` (this node was
inserted). Recovering the tree instead of failing the parse is what lets an editor keep
highlighting, folding, and navigating code that does not yet compile.

## Why matching C's recovery is hard

The parse tables come from the same upstream grammars C uses, so on valid input the LR automaton
is largely constrained to agree. Recovery is different. **C tree-sitter's recovery behavior is
essentially undocumented — it is not a specification anyone wrote down, it is whatever the C code
does.** No prose states "prefer skipping one token over inserting two." There is a cost model, a
version-competition loop, and a set of constants, and the emergent behavior of those pieces *is*
the reference.

So the reference is the C source, pinned to an exact version. gotreesitter's recovery core is
written in Go and follows the decision steps in the C functions that drive it —
`ts_parser__handle_error`, `ts_parser__recover`, `ts_parser__compare_versions`,
`ts_parser__condense_stack`, and their helpers. The cost constants follow C's `error_costs.h` (cost-per-recovery 500, cost-per-missing 110, cost-per-skipped-tree 100, and
similar values), and the maximum cost difference the version competition tolerates is pinned to
the v0.25.1 value, with a warning in the code *not* to "correct" it to the older v0.24 value —
doing so would silently break parity against the exact runtime this port was verified against.
That is how tight the coupling is: an off-by-one in a tie-break threshold is a divergence.

## How we verify: the instrumented-oracle method

The project uses differential tests against C, including checks that trace recovery decisions.
Here is the method:

1. **Compile C tree-sitter v0.25.1 as an oracle** and link it into a cgo test harness alongside
   the pure-Go engine. The harness parses the same bytes with both.
2. **Compare the trees node-by-node in lockstep.** The comparison is exact on the fields that
   define a tree: type, start byte, end byte, named-ness, missing-ness, child count, and field
   name. The first field that disagrees marks a divergence.
3. **Localize the first divergence.** A dedicated diagnostic parses one file with both engines,
   walks both trees together, and dumps the *first* structural difference with full sibling
   context — the exact node where Go and C part ways.
4. **Fix the divergence in the Go engine** so the decision matches C, add the reduced input as a
   permanent fixture, and repeat until the trees are identical.

Because recovery is a sequence of decisions, a single wrong choice cascades: skip the wrong token
and every node after it shifts. Catching the *first* point of disagreement — rather than
eyeballing two large trees — is what makes this tractable. The same lockstep walk runs across the
whole curated corpus, on fresh parses and on incremental reparses of edited source, always
against a C oracle built from the exact grammar commits the project pins. When the C source is
the only specification, replaying its decisions against an instrumented build is one way to
check a match.

## The election model

Reproducing C recovery decision-for-decision is expensive to *certify*, so it is enabled per
language rather than assumed everywhere. A language reaches the C-faithful recovery path only
when two independent conditions hold, tracked as two flags on the loaded `Language`:

- **Capability** — the grammar's tables actually expose the C recovery surface (RECOVER actions
  plus the error-state lex mode). This comes from reading the tables; it is metadata, not a
  promise of correctness.
- **Default certification** — the project has verified the faithful recovery loop as parity-safe
  for that language, and marks it on by default.

At runtime the gate additionally revalidates the table shape before engaging. Only when
capability *and* certification *and* runtime validation all pass does a parse run the C-faithful
loop; otherwise it falls back.

The tag does not publish a current aggregate election count. Inspect
`CRecoveryCostCompetitionCapable` and `CRecoveryCostCompetitionEnabledByDefault` on the loaded
language. Unelected grammars can use the resync fallback, whose error shape can differ from C.

One safety net captures the project's stance. A narrow language-agnostic check guards a specific
defect class in the port: if the version-competition step ever selects a *clean* final tree whose
own lineage had discarded recovery-owned content, that result is provably wrong — the C oracle
never emits a marker-free result from a version that needed recovery. When that exact signal
fires, the parse re-runs with the resync fallback and adopts its verdict. The guarantee is
deliberately scoped to "`HasError()` is honest," not "the shape is C-perfect" — and the code
documents that gap rather than hiding it.

## Published parity evidence

The v0.55.1 [curated gate ratchet](https://github.com/odvcencio/gotreesitter/blob/v0.55.1/cgo_harness/parity_gate_ratchet_test.go)
requires at least 206 structural languages, zero known-degraded structural entries, and zero
parity skips. Its highlight floor is 200. These are test thresholds, not a result for all inputs.

The [C parity boards](https://github.com/odvcencio/gotreesitter/blob/v0.55.1/docs/c-parity-boards.md)
publish separate, dated results. The figures below are the board's recorded results, not a new
sweep of the v0.55.1 tag:

| Board | Date | Published scope and result |
|---|---|---|
| Query semantics | 2026-09-08 | 103 cases; 101 agree; 2 known differences |
| Highlight parity | 2026-09-08 | 206 languages; 204 agree; 2 skip because hurl and mojo have no C reference build; 0 tolerance entries |
| Supertype maps | 2026-09-08 | 69 grammars; 40 agree; 29 differ |
| Recovery | 2026-09-19 | 79 cases; 39 agree on the default route after the leaf-extra fix |

The highlight skip count belongs to that board; it is separate from the curated gate's zero-skip
threshold. The v0.55.0 release notes record Python escape-span and list-splat fixes
([#1276](https://github.com/odvcencio/gotreesitter/pull/1276) and
[#1277](https://github.com/odvcencio/gotreesitter/pull/1277)). The v0.55.1 notes still list six
Django tree differences and open recovery gaps, including ERROR roots whose HasError value is
false ([#1280](https://github.com/odvcencio/gotreesitter/pull/1280)).

## Swift real-code corpus

The v0.48.0 corpus contains twelve pinned files from Swift 6.3 and
apple/swift-algorithms 1.2.1. Its expectation test rejects a regression and rejects an unrecorded
fix. It keeps the current result classification explicit.

Five Swift standard-library cases match the pinned C oracle but expose upstream grammar gaps.
Issues [#574](https://github.com/odvcencio/gotreesitter/issues/574) through
[#578](https://github.com/odvcencio/gotreesitter/issues/578) record them. Do not add a Go-only
repair for those cases. It would break locked C parity.

## Check correctness before timing

A parse can return without an error yet cover only part of the input. A timing result for
that tree does not establish correct parsing. Compare node types, fields, spans, flags, and
children against the pinned C oracle. Keep the correctness result with the performance evidence.

## What the results cover

The tests compare named structure, fields, spans, flags, and children on their recorded fixtures.
Some clean parses match C while recovery and other cases still differ. The recovery board's
published result is 39 matches among 79 cases (2026-09-19). The v0.55.1 release notes separately
list open gaps; neither set of results promises identical trees for all languages or inputs.
Keep each correctness result with its fixture, source revision, and oracle version.

> [!CAUTION] Don't read parity and speed as one signal
> For how the same discipline shows up on the speed axis — and where the honest performance
> asterisks are — see [Performance](/docs/performance).
