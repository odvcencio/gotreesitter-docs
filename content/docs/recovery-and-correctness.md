---
title: Recovery and Correctness
description: How gotreesitter reproduces C tree-sitter's parse trees byte-for-byte — including error recovery — and how that fidelity is kept true.
nav_group: Internals
order: 1
---

gotreesitter is a pure-Go reimplementation of tree-sitter, not a binding. Nothing
here calls into C at runtime. That raises an obvious question: how do you know a
from-scratch parser produces the *same* tree the C runtime would?

The answer this project holds itself to: **byte-for-byte identical trees, verified
against C tree-sitter v0.25.0** — and not just on well-formed code. **Error recovery
is included in that bar.** This page explains what that means, why it is hard, and
the machinery that keeps it honest.

## What error recovery is

Real code is frequently incomplete or invalid. An editor reparses on almost every
keystroke, so the parser spends much of its life looking at a half-typed function,
an unbalanced brace, or a paste that dropped a line. A parser that only accepted
grammatically perfect input would be useless for tooling.

tree-sitter never fails. When the input does not fit the grammar, it produces a
tree anyway, marking the damage with two special node kinds:

- **`ERROR`** — a span the parser could not fit into any grammar rule. Its children
  are the tokens it salvaged inside that span.
- **`MISSING`** — a zero-width node the parser *inserted* to complete a rule, when
  the grammar says a token should be there and it is not.

You can see both directly. Parse an unterminated JSON array:

```go
lang := grammars.JsonLanguage()
parser := gts.NewParser(lang)

tree, _ := parser.Parse([]byte(`[1, 2`))
fmt.Println(tree.RootNode().HasError()) // true
```

The tree gotreesitter returns for `[1, 2` is:

```text
document [0-5]
  array [0-5]
    [       [0-1]
    number  [1-2]
    ,       [2-3]
    number  [4-5]
    ]       MISSING [5-5]
```

The parser inserted a zero-width `]` at byte 5 so the `array` rule could close.
That is a *recovery decision*: which token to insert, where to resynchronize, how
much to wrap in `ERROR`. C tree-sitter makes the identical decision. So must
gotreesitter — the `MISSING ]` lands at byte 5, the `array` spans `[0-5]`, and the
child list matches, node for node.

At the API level, recovery surfaces through the node predicates:
`HasError()` (this subtree contains damage), `IsError()` (this node *is* an `ERROR`),
and `IsMissing()` (this node was inserted). Recovering the tree instead of failing
the parse is what lets an editor keep highlighting, folding, and navigating code
that does not yet compile.

## Why matching C's recovery is hard

The parse tables come from the same upstream grammars C uses, so on valid input the
LR automaton is largely constrained to agree. Recovery is different. **C
tree-sitter's recovery behavior is essentially undocumented — it is not a
specification anyone wrote down, it is whatever the C code does.** There is no
prose that says "prefer skipping one token over inserting two"; there is a cost
model, a version-competition loop, and a set of constants, and the emergent
behavior of those pieces *is* the reference.

So the reference is the C source, pinned to an exact version. gotreesitter's
recovery core is written as a deliberate port of the C functions that drive it —
`ts_parser__handle_error`, `ts_parser__recover`, `ts_parser__compare_versions`,
`ts_parser__condense_stack`, and their helpers — with the C code named as the spec.
The cost constants are copied from C's `error_costs.h` (cost-per-recovery 500,
cost-per-missing 110, cost-per-skipped-tree 100, and so on), and the maximum cost
difference the version competition tolerates is pinned to the v0.25.0 value with a
warning in the code *not* to "correct" it to the older v0.24 value — doing so would
silently break parity against the exact runtime this port was verified against. That
is how tight the coupling is: an off-by-one in a tie-break threshold is a divergence.

## How we verify: the instrumented-oracle method

The correctness discipline behind the byte-exact claim is differential testing
against C, driven down to individual decisions. The method:

1. **Compile C tree-sitter v0.25.0 as an oracle** and link it into a cgo test
   harness alongside the pure-Go engine. The harness parses the same bytes with
   both.
2. **Compare the trees node-by-node in lockstep.** The comparison is exact on the
   fields that define a tree: type, start byte, end byte, named-ness, missing-ness,
   child count, and field name. The first field that disagrees is a divergence.
3. **Localize the first divergence.** A dedicated diagnostic parses one file with
   both engines, walks both trees together, and dumps the *first* structural
   difference with full sibling context — the exact node where Go and C part ways.
4. **Fix the divergence in the Go engine** so the decision matches C, add the
   reduced input as a permanent fixture, and repeat until the trees are identical.

Because recovery is a sequence of decisions, a single wrong choice cascades: skip
the wrong token and every node after it shifts. Catching the *first* point of
disagreement — rather than eyeballing two large trees — is what makes this
tractable. The same lockstep walk runs across the whole curated corpus, on fresh
parses and on incremental reparses of edited source, always against a C oracle built
from the exact grammar commits the project pins. When the C source is the only
specification, replaying its decisions against an instrumented build is the only way
to know you match it.

## The election model

Reproducing C recovery decision-for-decision is expensive to *certify*, so it is
enabled per language rather than assumed everywhere. A language reaches the
C-faithful recovery path only when two independent conditions hold, tracked as two
flags on the loaded `Language`:

- **Capability** — the grammar's tables actually expose the C recovery surface
  (RECOVER actions plus the error-state lex mode). This is read from the tables; it
  is metadata, not a promise of correctness.
- **Default certification** — the faithful recovery loop has been verified
  parity-safe for that language and is marked on by default.

At runtime the gate additionally revalidates the table shape before engaging. Only
when capability *and* certification *and* runtime validation all pass does a parse
run the C-faithful loop; otherwise it falls back.

About **124 languages are elected** into C-faithful recovery today (123 as of
v0.21.0, plus C# added in a later wave). For the remainder, gotreesitter uses a
**resync recovery path** — a coarser, structurally safe fallback that predates the
default-on C recovery. It never fails a parse and never hangs, but it can wrap a
larger span in `ERROR` than C's local cost competition would; the tree is safe to
consume, not necessarily byte-identical to C on that damaged region. The rest of
the long tail is staged for election as each language is verified.

One safety net captures the project's stance. A narrow language-agnostic check
guards a specific defect class in the port: if the version-competition step ever
selects a *clean* final tree whose own lineage had discarded recovery-owned content,
that is provably wrong — the C oracle never emits a marker-free result from a version
that needed recovery. When that exact signal fires, the parse re-runs with the
resync fallback and adopts its verdict. The guarantee is deliberately scoped to
"`HasError()` is honest," not "the shape is C-perfect" — and the code documents that
gap rather than hiding it.

## The gates that keep it true

Parity is not a one-time achievement; it is a ratchet. The cgo parity suites
(`TestParityFreshParse`, `TestParityIncrementalParse`, `TestParityHasNoErrors`, the
GLR canary and cap-pressure suites) run the node-exact comparison against the C
oracle in CI. A coverage-ratchet test locks the gate so it can only tighten: the
curated structural set must stay at **206 languages**, the known-degraded structural
list may not grow past **14**, highlight coverage stays at 200 with its own
degraded ceiling, and the count of tolerated parity skips is pinned at **zero**.
Loosening any of those numbers requires editing the ratchet on purpose, in the open.

A separate tier model scores every grammar on real-corpus files, with a hard rule
stated in the tooling: **parity versus the C oracle is the gate; performance is only
a sub-rank.** A grammar that regresses below its recorded parity floor fails the
tier ratchet. Correctness and speed are deliberately kept as separate gates — the
harness framework spells it out: *do not infer correctness from performance numbers.*

## The campaign lesson: cliffs are correctness bugs in disguise

The most important thing this project learned about correctness came from chasing
*performance*. Several dramatic "performance cliffs" turned out not to be slow code
— they were **wrong** code that happened to be slow.

- A **bash** file took **46.4 seconds** and came back wrapped in a whole-file
  `ERROR`. A profiler pointed at the parser working furiously. The real cause was a
  correctness bug: bash program bodies are long repeated statement lists, and the
  engine was forking `{shift, reduce}` at *every* statement boundary instead of
  folding the list spine, so it reached end-of-input with no accepting lineage and
  declined into a whole-file error. Fixed to fold the spine the way C does, the file
  parses in about **8 ms** and produces the C-oracle shape. A regression test now
  pins the exact child count of that recovered tree.
- **kotlin** silently mislexed every safe-navigation `?.` operator — an external
  scanner bound to the wrong symbol produced a spurious `ERROR`, which sent
  recovery into O(n²) work. It read as a speed problem; it was a lexing bug.
  Correcting the binding took one large file from **5,036 ms to 74 ms**, at
  node-for-node C parity.
- A cluster of languages (**cpp, haskell, scala, crystal, typescript**) showed a
  subtler shape: a parse that returns *without* error and *without* stopping early,
  but whose root covers only a fraction of the input — a silently truncated tree. A
  timing sweep happily reports such a file as "fast." Only comparing against the
  oracle reveals that the tree is wrong.

The through-line: a profiler shows you where time goes, not whether the work is
legitimate. A parser burning cycles might be correctly parsing a hard file, or it
might be thrashing because it already made a wrong decision and cannot recover.
**Only oracle discipline distinguishes the two.** Every one of these was found by
comparing to C, not by reading a flame graph. Related wins from the same campaign —
php going from 8/8 timeouts to 8/8 clean parses at \~1.66× C, rust from \~71.7 s to
\~2.4 s, java from \~5.19 s to \~521 ms on the worst sampled files — trace back to the
same habit of treating a divergence as a bug until proven otherwise.

## Honest scope

Byte-exact parity against C is **verified**, not assumed, and the scope is finite:
it holds across the curated parity corpus and for the elected languages, checked
fresh and incrementally against the pinned C oracle in CI. On the broader
real-corpus sweep — dozens of files per language, drawn from upstream repositories —
the long tail is genuinely still being closed: some languages are fully
parity-clean, others have known divergences that are tracked, ratcheted, and worked
down one reduced fixture at a time. The claim is not "every tree from every input is
identical to C." It is "where a language is elected and on the corpus we test, the
tree matches C byte-for-byte, including recovery — and the gates make it expensive to
quietly lose that."

> [!CAUTION] Don't read parity and speed as one signal
> For how the same discipline shows up on the speed axis — and where the honest
> performance asterisks are — see [Performance](/docs/performance).
