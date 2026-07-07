---
title: Architecture
description: How gotreesitter's GLR engine, C-faithful recovery, GSS-forest fast path, and two-layer grammar system fit together — a map for contributors.
nav_group: Internals
order: 3
---

gotreesitter is a ground-up reimplementation of the tree-sitter runtime in Go. No code is shared
with or translated from the C implementation — the lexer, GLR parser, incremental engine, arena
allocator, and query engine are each written against tree-sitter's *behavior*, checked
decision-by-decision against a real C build rather than against its source. This page is a map:
what the pieces are, how a parse moves through them, and which files to open first.

## Two layers: engine and grammar data

gotreesitter splits into two layers that happen to live in one Go module.

The **engine** — the root package `github.com/odvcencio/gotreesitter` (`parser.go`, `glr.go`,
`lexer.go`, `tree.go`, `arena.go`, and around it) — knows nothing about Go-the-language or
Python-the-language. It only knows `Language` (`language.go`), a struct mirroring tree-sitter's
`TSLanguage`: parse-action tables, a lexer DFA, field maps, symbol metadata, and an optional
external scanner. Hand it any well-formed `Language` and it can parse it.

The **grammar data** — `github.com/odvcencio/gotreesitter/grammars` — is 206 of those `Language`
values, one per supported language, plus the machinery to load them lazily. `grammars.GoLanguage()`,
`grammars.PythonLanguage()`, and 204 more `XLanguage()` functions each lazily decode one compressed
blob under `grammars/grammar_blobs/*.bin` (`loadEmbeddedLanguage`, `grammars/embedded_loader.go`),
cached behind an LRU (`GOTREESITTER_GRAMMAR_CACHE_LIMIT`) so a program that only calls
`GoLanguage()` never pays to decode the other 205.

The dependency runs one direction only: nothing in the engine imports `grammars`. What the grammars
package adds — external scanners, hand-written token sources, preferred-language overrides — plugs
into the engine through public interfaces (`ExternalScanner`, `TokenSource`) and struct fields
(`Language.ExternalScanner`).

## Making a parse table

Where do the 206 `Language` values come from? Two pipelines feed the same target shape.

`ts2go` (`cmd/ts2go`) reads a tree-sitter-generated `parser.c` — the output of upstream
`tree-sitter generate` — and translates its parse tables, lex tables, and field maps into Go. This
produces most of the 206 embedded grammars today (see [Contributing](/docs/contributing) for the
exact command).

`grammargen` (the `grammargen` package, driven by `cmd/grammargen`) is gotreesitter's own grammar
compiler: a pure-Go LALR/LR(1) table builder with a targeted state-splitting pass
(`grammargen/lr_split.go`) and its own lexer-DFA construction (`grammargen/dfa.go`). It never
shells out to the tree-sitter CLI. It accepts a resolved `grammar.json`
(`grammargen.ImportGrammarJSON`), a hand-authored Go DSL grammar, or a compact `.grammar` file, and
emits a `*gotreesitter.Language`, a blob, generated Go DSL source, or even a tree-sitter-compatible
`parser.c` (`grammargen/encode.go`). It's also creeping into the embedded set itself: Go's own blob
is grammargen-compiled now, closing a dead-end parse state the original tree-sitter-go grammar
carried.

Both pipelines converge on the same `Language` shape and the same blob format, so the runtime
doesn't care which one produced a given grammar. That convergence is also the no-fork extension
path: `ImportGrammarJSON` → `GenerateLanguageAndBlob` → `grammars.Register` /
`grammars.RegisterExtension` lets a consumer add or override a language from their own module
without forking gotreesitter — see [Authoring Languages](/docs/authoring-languages).

## Anatomy of a parse

```go
lang := grammars.GoLanguage()
parser := gts.NewParser(lang)

tree, err := parser.Parse(src)
if err != nil {
    panic(err)
}

tree.Edit(edit) // an InputEdit describing the changed byte range
newTree, err := parser.ParseIncremental(newSrc, tree)
if err != nil {
    panic(err)
}
fmt.Println(newTree.RootNode().HasError())
```

`Parser.Parse` (`parser_api.go`) wraps one call: `p.parseInternal` (`parser.go`), documented in the
source as "the core GLR parsing loop shared by `Parse` and `ParseWithTokenSource`." `Parse` first
tries a fast path (the GSS-forest, below); if that declines, `parseInternal` runs the full loop:

1. **Tokens.** A `TokenSource` (one method, `Next() Token`) feeds the loop. Most grammars use the
   generic DFA walker (`dfaTokenSource`, `parser_dfa_token_source.go`), executing the lex DFA table
   baked into the `Language` ahead of time. Grammars whose tokens need context a DFA can't express —
   Python's indentation, Go's automatic semicolons, YAML's nesting — supply a hand-written Go
   `TokenSource` instead (7 shipped, e.g. `grammars.AuthzedTokenSource`), and 116 grammars
   additionally attach an `ExternalScanner` (`Create`/`Destroy`/`Serialize`/`Deserialize`/`Scan`) for
   individual context-sensitive tokens inside an otherwise DFA-driven grammar.
2. **GLR dispatch.** Each token drives a lookup in `Language.ParseActions`. One action, one stack —
   ordinary LR. More than one action and the parser forks (next section).
3. **Tree.** Surviving stacks accumulate `Node`s (`tree.go`) out of a slab arena (below).
   `parser_result_root_build.go` picks or assembles the final root from whatever stacks are still
   alive at EOF.
4. **Result.** A post-build pass (`normalizeResultCompatibility`, `parser_result_compat.go`)
   dispatches by language name to one of the `parser_result_<lang>.go` files for, in the source's
   own words, "narrow post-build tree rewrites that keep gotreesitter output aligned with C
   tree-sitter" — field attachment, alias flattening, a handful of per-language shape fixes — before
   the `*Tree` goes back to the caller.

`ParseIncremental` skips most of that. A `reuseCursor` (`incremental.go`) walks the edit region of
the old tree in pre-order and splices in untouched subtrees by reference. Each interior node
carries the parser state it had *before* its children were reduced (`Node.preGotoState`), so the
parser can skip a whole reused subtree and resume in the right state without re-deriving its
contents. A narrower fast path (`tryTokenInvariantLeafEdit`, `incremental_leaf_fastpath.go`)
recognizes edits that don't shift any token boundary and patches a single leaf directly — this is
the path behind the 0-allocation incremental numbers on the [Introduction](/docs/introduction) page.

## The GLR core and the graph-structured stack

`Language.ParseActions` maps every `(state, symbol)` pair to a `ParseActionEntry` holding one or
more `ParseAction`s (`language.go`). When there's more than one — the grammar is locally ambiguous
at that point — the loop in `parser.go` forks: one `glrStack` (`glr.go`) per alternative, each
explored independently. A stack that hits a dead end is dropped; the rest keep going.

Forking doesn't mean copying the whole stack forever. Stacks share a graph-structured stack (GSS):
`gssStack`/`gssNode` (`glr_gss.go`) let a fork share the unchanged tail of its parent and only
diverge where it actually differs, and stacks that land back on the same `(state, position)` merge
instead of being explored as separate universes forever (`mergeKeyForStack`, `stackCompareMerge`,
both in `glr.go`). Safety caps — iteration count, stack depth, and node count, all scaled to input
size, plus a hard ceiling on live stack versions (`GOT_GLR_MAX_STACKS`, default 8) — stop a
pathologically ambiguous grammar from forking forever and force a clean stop instead.

For grammars whose ambiguity produces a lot of *dead-end* forking — the same subtree rebuilt and
deep-compared for equivalence over and over — `glr_forest.go` adds an alternative representation
where one GSS node holds several `(state, position)` alternatives as forest links instead of one
tree per stack, closer to how C tree-sitter's graph-structured stack behaves internally. It's an
opt-in fast path, curated per grammar (`builtinForestDefaults`) or opted into directly
(`Language.WantsForest`), that declines back to the ordinary GLR path on anything but a clean,
complete parse — it has no recovery of its own, though a smaller curated subset
(`languageWantsForestRecover`) also carries a narrower forest-side recovery path certified against
the C oracle. A language joins the default set only once its output is checked byte-range-identical
to the ordinary path (or to C) on real corpus; where it applies, the payoff is large — measured
speedups from 3× to over 800× on the languages hit hardest by stack-merge blowup.

## Recovery: the C-faithful port, and the fallback

When the lookahead has no action at all, tree-sitter's C runtime doesn't just error out — it runs a
specific, documented algorithm to recover (`parser.c`'s `handle_error`/`recover`/
`condense_stack`, an error-cost model, and pruning to the best few candidate versions).
`parser_recover_c.go` ports that algorithm decision-by-decision rather than approximating it — its
header states the discipline outright: "THE C CODE IS THE SPEC," followed by a literal map from
each C function to its Go counterpart. Absorbing stacks pause instead of dying (mirroring C's
`StackStatusPaused`), error cost accrues per stack using the same constants as C's `error_costs.h`,
and the best-scoring version resumes through the same condense step C uses (`cCondenseAndResume`,
porting `ts_parser__condense_stack`).

This path is gated per grammar: `Language.CRecoveryCostCompetitionCapable` records that a grammar's
tables carry the RECOVER/ERROR_STATE surface the port needs, and
`CRecoveryCostCompetitionEnabledByDefault` records that it's certified safe as default behavior.
Elected grammars get recovery decisions matched to C's; the rest fall back to a simpler panic-mode
resync (`tryResyncErrorRecoveryMode`, `parser_reduce.go`) that pops to the top-level state, wraps
the damage in one `ERROR` node, and resumes — safe and bounded, but not decision-matched to C.

## Arenas: why the hot path allocates nothing

Every `Node` comes from a slab-backed `nodeArena` (`arena.go`), never an individual heap
allocation. Full parses and incremental edits draw from differently-sized slab classes
(`arenaClassFull` vs. `arenaClassIncremental`), arenas are ref-counted so a tree that borrows reused
subtrees from an old tree keeps that arena alive exactly as long as needed, and arenas are pooled
and reused across parses instead of being handed back to the garbage collector every time. That
discipline is what makes a no-op incremental reparse a genuine 0 bytes / 0 allocations operation
rather than just a small one: the hot path has nothing left to allocate.

## Finding your way around

| Area | Start here |
|---|---|
| GLR core: forking, merging | `glr.go`, `glr_gss.go`, `glr_forest.go` |
| Error recovery | `parser_recover_c.go`, `parser_reduce.go`, `parser_recovery.go` |
| Lexing | `lexer.go`, `parser_dfa_token_source.go`, `external_lexer.go` |
| Incremental reparsing | `incremental.go`, `incremental_leaf_fastpath.go` |
| Memory | `arena.go` |
| Tree / public API | `tree.go`, `parser_api.go`, `cursor.go` |
| Per-language result shaping | `parser_result_compat.go`, `parser_result_<lang>.go` |
| Query engine | `query.go`, `query_compile.go`, `query_matcher.go` |
| Highlighting, injection, rewriting | `highlight.go`, `injection.go`, `rewrite.go` |
| Grammar compiler | `grammargen/` (`lr.go`, `lr_split.go`, `dfa.go`, `encode.go`) |
| C-table extraction | `cmd/ts2go/` |
| Grammar data, loading, registry | `grammars/` (`embedded_loader.go`, `registry.go`) |

Changing GLR or recovery behavior? Read [Contributing](/docs/contributing) next — this codebase
gates those changes against a real C tree-sitter build, not just its own test suite.
