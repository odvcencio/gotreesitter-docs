---
title: "Pure-Go Tree-sitter: from zero to self-hosted infra"
theme: swiss
transition: fade
line-numbers: false
event: "GopherCon 2026"
---

```yaml
layout: title
accent: "var(--color-pink)"
```

> GopherCon 2026 · Aug 5 · 25 min

# Pure-Go Tree-sitter: **from zero to** *self-hosted infra*

Oracles, bench gates, and the story of learning how to trust a runtime an AI
helped write.

- no CGo
- 206 grammars
- byte-exact C parity
- AI-assisted

<DeckStyle/>

<Notes>
25 minutes. Three threads: the parser itself, the discipline that let AI build
it, and the platform that fell out. Press o for overview, p for presenter view.
</Notes>

---

> The project

# The constraint

tree-sitter gives editors and agents a fast, incremental, multi-language
parse. The catch is **CGo**: a C compiler on every build, on every target.

- Cross-compilation needs a C cross-toolchain per target
- `go install` fails for anyone without a C compiler
- Race detector, coverage, and fuzzer are blind across the FFI boundary

```sh
$ GOOS=js GOARCH=wasm go build ./...
$ GOOS=wasip1 GOARCH=wasm go build ./...
# no cc, no toolchain, no cgo
```

<Notes>
Why this project exists. Every Go tree-sitter binding depends on CGo:
cross-toolchains per target, gcc in CI, go install breaks downstream, and the
race detector can't see across the FFI boundary.
</Notes>

---

> The project

# The pipeline

1. **grammar blob** ts2go extract · grammargen compile
2. **pure-Go parser** LR + GLR runtime
3. **forest** forks tracked
4. **tree** normalized to ts shape
5. **queries** highlights · tags · captures
6. **consumers** editors & agents

> Two ways in: **ts2go** extracts tables from upstream parser.c, and **grammargen** — a self-hosting LALR(1) compiler — builds them from grammar.json in pure Go. No C anywhere in the loop.

<Notes>
Nothing here invents a new grammar format — but there are two ways in: ts2go
extracts tables from upstream parser.c, and grammargen compiles grammar.json
to tables itself, self-hosted, pure Go — it can even generate tables in the
browser. The runtime — parser, lexer, queries, incremental, scanners — is all
Go.
</Notes>

---

> The project

# The scale

- **206** grammars in the registry
- **116** external scanners, rewritten in Go
- **156/156** highlight queries compile
- **1** human on the project

Five months of work, now self-hosting with its own LALR(1) grammar compiler.
The rest of this talk is **how** — and how to know the code is actually
getting better.

<Notes>
Set the scale before the how. This is not a toy: 206 grammars, 116
hand-written Go external scanners, every shipped highlight and tags query
compiles. One person plus agents.
</Notes>

---

```yaml
layout: section
background: "var(--color-ink)"
accent: "var(--color-pink)"
```

> 01

# Harness before cannon

Much of the implementation was AI-assisted. The interesting part isn't that an
LLM wrote code — it's the harness that made the code trustworthy.

<Notes>
Act one: the discipline. The instinct with a strong model is to start
generating code on day one. The actual first artifact was the harness.
</Notes>

---

> Part 1 · Discipline

# The rule

Never point an agent at a problem *you can't mechanically verify*.

- A model will make a test pass the wrong way if the test lets it
- Nobody reviews 10,000 generated lines of GLR logic by eye
- Every bug becomes a reproduction test; every claim carries a receipt
- So the first build artifact was the **failure surface**, not the parser

<Notes>
An agent will happily 'fix' a parser by making the test pass in the wrong way.
You cannot review ten thousand generated lines of GLR logic by eye — the
failure surface has to be mechanical.
</Notes>

---

> Part 1 · Discipline

# The oracle

Ground truth isn't a spec or an opinion — it's **tree-sitter C itself**,
pinned and fingerprinted.

```sh
# one grammar, one container, one verdict
$ bash cgo_harness/docker/run_single_grammar_parity.sh typescript

# oracle: tree-sitter v0.25.1 @ f5afe475…
# compiled -O2, static, fingerprinted
```

- **Byte-exact, not "close"** Admission requires exact deep parity: the same
  tree the C runtime selects, on real corpora.
- **Isolated verdicts** One language per Docker container — no cross-grammar
  noise, no OOM ambiguity, reproducible on any machine.

<Notes>
Ground truth is upstream tree-sitter C — pinned commit, compiled -O2 into a
fingerprinted static artifact. Parity means the exact tree C selects, byte for
byte. Correctness admission is deep-equal against that oracle, one grammar per
Docker container.
</Notes>

---

> Part 1 · Discipline

# The gates

Correctness, parity, and performance gate **separately**. A change clears all
three or it doesn't land.

- **1 · Correctness** Focused unit + package tests, scoped with `-run`, inside
  Docker.
- **2 · Parity** C-oracle suites per grammar. GLR or incremental change?
  Parity validates first.
- **3 · Performance** `GOMAXPROCS=1 · count=10 · benchstat`. Keep the change
  only if benchstat shows a real win.

> The agent doesn't grade its own work. **benchstat** and the parity lane do.

<Notes>
Three gates, kept separate on purpose. An agent-proposed change touches parity
first, then perf under pinned settings. benchstat decides, not the agent's
summary of itself.
</Notes>

---

> Part 1 · Discipline

# The audits

The harness catches **you** too. An audit found the headline benchmark
measured a no-tree diagnostic path — so the numbers were withdrawn.

- **Withdrawn** The published `1.54 ms` full parse and `1.895x` C ratio —
  parser-core diagnostics, not a materialized parse.
- **Replaced** A locked matrix of real, human-authored Go files against the
  fingerprinted C oracle: `5.48x` geomean, every receipt in `BENCH.md`.

Honest numbers are a testing mechanism. If the benchmark can lie, the agent
will optimize the lie.

<Notes>
The harness catches you too. A 2026 audit found the headline benchmark was
measuring a no-tree diagnostic path. The 1.54ms and 1.895x numbers were
withdrawn publicly and replaced with a locked real-code baseline — receipts in
BENCH.md. That's the same discipline applied to the human.
</Notes>

---

```yaml
layout: section
background: "var(--color-ink)"
accent: "var(--color-pink)"
```

> 02

# Growing through model releases

The harness stayed. The models got stronger. With each release, the same gates
admitted bigger changes.

<Notes>
Act two: the same harness, successive models. What changed between model
releases wasn't the process — it was how much the gates would admit per unit
of my attention.
</Notes>

---

> Part 2 · The loop

# The contract

`AGENTS.md` at the repo root tells every agent — and every new model — exactly
how to work here.

```md
## Non-negotiables
- no `go test ./...` on the host
- one language per container
- parity validation before perf
- correctness gates != perf gates

## Commit discipline
- scoped, bisectable commits
- buckley commit --yes --minimal-output
```

- **Gate presets** Every check is one command. The agent never invents its own
  verification path.
- **Model-portable** Swap the model, keep the contract. A new release is
  productive in its first session.

<Notes>
AGENTS.md is the contract at the repo root: non-negotiables, gate presets,
commit discipline. The agent reads it before touching anything. Commits go
through buckley. This file is what makes a new model productive on day one.
</Notes>

---

> Part 2 · The loop

# The trajectory

1. *early releases* **Breadth** 206 grammars registered, 116 external scanners
   ported to Go, smoke parity across the registry — mechanical volume the
   gates could absorb.
2. *mid releases* **Real optimization** GLR fork collapse (PR #90): JavaScript
   wall ratio `5.95x → 4.41x` — verified per-state against the C oracle's goto
   chains.
3. *v0.37* **Architecture pivots** GLR materialization v2, the locked static-C
   oracle, authenticated real-code baselines. The kind of change you'd never
   let an unverified agent near.

<Notes>
The trajectory across releases — customize the model names to your actual
history. Early: breadth, smoke parity, scanner ports. Mid: real GLR
optimization like the fork-collapse work. Recent: architecture pivots like
materialization v2 and the locked oracle publication in v0.37. Same gates
throughout.
</Notes>

---

```yaml
background: "var(--color-ink)"
accent: "var(--color-green)"
```

> Part 2 · Evidence

# The evidence

Collapse GLR forks that C resolves deterministically — zero parity risk,
because **C itself proves the action**.

<Benchmark Before={5.95} After={4.41} Baseline={1.00}/>

JavaScript grammar, Go-vs-C wall ratio — PR #90, gated by parity + benchstat

the trade: full parses give up raw speed for portability; incremental editor
workloads beat the C path

<Citation href="https://github.com/odvcencio/gotreesitter/pull/90" label="PR #90 · JavaScript GLR fork reduction"/>

<Notes>
One concrete artifact of the loop. Fork count is the first-order lever;
collapsing forks C resolves deterministically is zero parity risk because C
itself proves the action. Proposed by an agent, admitted by the gates.
</Notes>

---

```yaml
layout: section
background: "var(--color-ink)"
accent: "var(--color-pink)"
```

> 03

# An agent-native platform

The workflow generalized. The tools that ran gotreesitter became
infrastructure.

<Notes>
Act three: the tooling that grew around the workflow. Once the loop worked for
one repo, the pieces generalized into a platform agents are native to.
</Notes>

---

> Part 3 · Platform

# The pieces

- **canopy** Structural analysis — inspection, impact, hotspots. Agents query
  structure, not grep.
- **graft** Structural version control — entity-level diff, merge, and commit
  indexing on Git.
- **buckley** The agent harness — runs sessions, owns the commit flow. Named
  after the dog.
- **hyphae** Shared memory — the knowledge graph agents report into and reason
  from.
- **arbiter** Governed outcomes — a compact language for what agents may and
  may not do.
- **sirena** Diagrams as projections — gotreesitter reads code, agents emit
  regenerable views.

> Several of these are built **on gotreesitter** — the parser feeds its own platform.

<Notes>
Each tool covers one part of the loop: canopy for structural analysis instead
of grep; graft for entity-level diff/merge; buckley runs the sessions and the
commits; hyphae is the shared memory agents report into; arbiter governs
outcomes; sirena regenerates diagrams from code. Several are themselves built
on gotreesitter — the parser feeds its own platform.
</Notes>

---

> Part 3 · Platform

# The factory

One grammar-surface substrate. **Eight products since February 2026.**

- **gosx** 3D-native web framework — it renders this deck
- **cobalt** COBOL modernization via grammar composition
- **manta** GPU inference language
- **ferrous-wheel** Rust-inspired syntax sugar for Go
- **fyx** Scripting language for the Fyrox engine
- **arbiter** Compact language for governed outcomes
- **danmuji** Streaming comment overlays
- **mdpp** Markdown++ — the format this deck is written in

<Notes>
The payoff of the platform: a shared grammar-surface factory. Eight products
shipped since February 2026, most of them built on gotreesitter — including
gosx, which renders these slides.
</Notes>

---

> Part 3 · Platform

# The adoption

Zero promotion. Engineers — and their AI assistants — found it, evaluated it,
and **built on it in public**.

1. **99** public source importers
2. **8** product categories
3. **5** months of work

- **OpenTendril** — replaced containerized parsing, gated by its own parity
  tests
- **tld · Stacklit · RepoNerve** — architecture diagrams, repo maps, and code
  memory for agents
- **QML Language Server** — a full LSP with no CGo anywhere

<Notes>
Zero marketing — engineers and their AI assistants find it, evaluate it
against product needs, and make public architectural decisions. OpenTendril
replaced containerized parsing with it as the default engine, gated by their
own parity tests. And my users have users: these products ship binaries whose
end users run gotreesitter without knowing its name.
</Notes>

---

> Part 3 · Platform

# Agent-native, concretely

You can start this in any repo, tomorrow:

- **A contract at the root** — AGENTS.md: non-negotiables, presets, commit
  flow
- **One-command gates** — every verification is a single reproducible script
- **An oracle** — a mechanical ground truth the agent can't argue with
- **Structure over text** — give agents parse trees and captures, not grep
- **Durable memory** — decisions land in notes agents read next session
- **Receipts** — every public claim traces to a pinned, rerunnable run

<Notes>
What agent-native means concretely, in any repo, starting tomorrow. None of
this requires the platform — AGENTS.md, one-command gates, and an oracle are
enough.
</Notes>

---

> Close

# Takeaways

1. **Harness before cannon.** Build the failure surface before you let an
   agent build the system.
2. **Gates outlive models.** A good contract makes every model release an
   upgrade, not a rewrite.
3. **Agent-native is a repo property.** Contracts, one-command gates, oracles,
   durable memory — it works in any codebase.

<Notes>
Land the three threads as one idea each. Pause on each.
</Notes>

---

```yaml
layout: title
background: "var(--color-ink)"
accent: "var(--color-green)"
```

> The close

# **Make parity** boring

When parity is boring, language tooling becomes **portable infrastructure** —
and agents become safe contributors to it.

- go get github.com/odvcencio/gotreesitter
- gotreesitter.m31labs.dev

## thank you · questions?

<Notes>
Close on portability and discipline. Everything downstream gets to depend on a
pure-Go parser substrate — and stop thinking about it. Q&A.
</Notes>
