---
title: Parsers in Depth
description: The Parser control surface most integrations need eventually — timeouts, cancellation, strict vs partial-tree parsing, debug logging, option-based parses, and a concurrency-safe pool.
nav_group: Using the Parser
order: 8
---

The [getting-started guide](/docs/getting-started) uses `Parser.Parse` and moves on — for a batch tool over trusted files, that is the whole story. Integrations that run on untrusted input, or on every keystroke, eventually need the rest of the `*Parser` control surface: a timeout so one pathological file can't wedge a request, a cancellation flag wired to a `context`, strict variants that turn a partial parse into an error, debug hooks, and a concurrency-safe pool. All of it hangs off the plain `*Parser` you get from `gts.NewParser(lang)`.

## The Parse family

`Parser.Parse(source []byte) (*Tree, error)` is the workhorse. Around it is a matrix of variants along four independent axes; pick the one method that combines the axes you need.

| Axis | Default (`Parse`) | Variant |
|---|---|---|
| Encoding | UTF-8 | `ParseUTF16`, `ParseUTF16Bytes` |
| Reuse | full parse | `ParseIncremental(source, oldTree)` — see [Incremental Parsing](/docs/incremental-parsing) |
| Lexer | built-in DFA | `ParseWithTokenSource(source, ts)` — a custom lexer bridge |
| Early stop | partial tree + `nil` error | `ParseStrict` and every `…Strict` sibling |

The combinations are spelled out as concrete methods — `ParseIncrementalWithTokenSourceStrict`, `ParseUTF16BytesWithTokenSourceFactory`, and so on — so a call never carries a `nil` for an axis that doesn't apply. To compose the axes as options instead of reaching for a long method name, use [`ParseWith`](#option-based-parsing-parsewith) below.

## Timeouts

`SetTimeoutMicros(microseconds uint64)` bounds a single parse. A value of `0` (the default) disables the check.

```go
parser := gts.NewParser(lang)
parser.SetTimeoutMicros(50_000) // 50 ms ceiling

tree, err := parser.Parse(src)
if err != nil {
    log.Fatal(err)
}
defer tree.Release()

if tree.ParseStoppedEarly() && tree.ParseStopReason() == gts.ParseStopTimeout {
    // `tree` is a real, usable *partial* tree covering the prefix that
    // parsed in time — not nil, not an error.
}
```

The load-bearing semantic, faithful to upstream tree-sitter: **a timeout is not an error.** `Parse` returns a tree and a `nil` error; the tree covers whatever was accepted before the deadline, and you detect the early stop through `tree.ParseStoppedEarly()` / `tree.ParseStopReason()`. If you would rather have the timeout come back as an `error`, use the [strict variants](#strict-parsing-partial-trees-vs-errors).

Timeouts are checked cooperatively — the parser stops at its next check point, not at an exact microsecond.

## Cancellation

`SetCancellationFlag(flag *uint32)` points the parser at a caller-owned flag. Parsing stops as soon as the pointed-to value becomes non-zero — set it from another goroutine, a `context` watcher, or a deadline timer.

```go
var cancel uint32
parser.SetCancellationFlag(&cancel)

go func() {
    <-ctx.Done()
    atomic.StoreUint32(&cancel, 1)
}()

tree, _ := parser.Parse(src)
defer tree.Release()

if tree.ParseStoppedEarly() && tree.ParseStopReason() == gts.ParseStopCancelled {
    // caller cancelled; `tree` is the partial result so far
}
```

Like a timeout, cancellation yields a partial tree with a `nil` error by default; the strict variants turn it into an error. And like a timeout, it is cooperative — it ends the parse at the next check, not instantly.

Timeouts and cancellation are two of several reasons a parse can stop early. `ParseStopReason()` also reports resource limits — `ParseStopNodeLimit`, `ParseStopMemoryBudget`, `ParseStopStackDepthLimit` — through the same channel, so one `ParseStoppedEarly()` check covers all of them.

## Strict parsing: partial trees vs errors

By default gotreesitter mirrors tree-sitter: a parse that stops early — timeout, cancellation, a resource limit, or exhausting recovery — still returns a usable partial tree and a `nil` error, and you opt into inspecting `tree.ParseStoppedEarly()`. When *partial is not acceptable* and any early stop should be a failure, use the strict variants:

```go
tree, err := parser.ParseStrict(src)
if errors.Is(err, gts.ErrParseStoppedEarly) {
    // parsing did not accept all input; `tree` still holds the partial result
}
```

`ParseStrict` returns `ErrParseStoppedEarly` instead of a silently-partial tree (the partial tree is still available). Every parse method has a `…Strict` sibling — `ParseIncrementalStrict`, `ParseWithTokenSourceStrict`, `ParseWithStrict` — so you choose the error contract independently of the other axes.

## Option-based parsing: ParseWith

If the method matrix feels like a lot of names, `ParseWith` collapses the axes into composable options and returns a structured result:

```go
result, err := parser.ParseWith(src,
    gts.WithOldTree(oldTree), // incremental reuse
    gts.WithProfiling(),      // per-edit attribution (incremental only)
)
if err != nil {
    log.Fatal(err)
}
tree := result.Tree
defer tree.Release()

if result.ProfileAvailable {
    fmt.Println(result.Profile) // which subtrees were reused vs re-parsed
}
```

The options are `WithOldTree(*Tree)` (incremental reuse), `WithTokenSource(TokenSource)` (custom lexer), and `WithProfiling()` (populate `ParseResult.Profile`). Profiling attribution is currently produced only for incremental parses; `ParseResult.ProfileAvailable` tells you whether it is present. `ParseWithStrict` is the strict counterpart. The return is a `ParseResult{Tree, Profile, ProfileAvailable}` rather than a bare `*Tree`.

## Debugging a parse

Three hooks, all off by default, all for diagnostics rather than production:

- **`SetLogger(fn ParserLogger)`** — `ParserLogger` is `func(kind ParserLogType, message string)`. It receives the parser's internal event stream: `kind` is `ParserLogLex` for token-source events and `ParserLogParse` for parse-loop control flow (shifts, reduces, recovery). Pass `nil` to disable; `Logger()` reads back the current one.

  ```go
  parser.SetLogger(func(kind gts.ParserLogType, msg string) {
      log.Printf("[%v] %s", kind, msg)
  })
  ```

- **`SetGLRTrace(true)`** — verbose GLR stack tracing to stdout. Useful when a grammar forks and you want to watch the [graph-structured stack](/docs/architecture) evolve; noisy enough that it is stdout-only and debug-only.
- **`SetAmbiguityProfile(*AmbiguityProfile)`** — installs a counter sink that tallies parser state, lookahead, and action counts, for measuring how GLR-heavy a workload is. Pass `nil` to disable.

## Concurrency: ParserPool

A `*Parser` is **not** safe for concurrent use — it carries mutable per-parse state (the arena, cursors, the timeout and cancellation config). Do not share one across goroutines. To serve many parses at once, use `ParserPool`, which is concurrency-safe:

```go
pool := gts.NewParserPool(lang,
    gts.WithParserPoolTimeoutMicros(50_000),
)

// From any number of goroutines:
tree, err := pool.Parse(src)
if err != nil {
    log.Fatal(err)
}
defer tree.Release()
```

Each `pool.Parse` (or `pool.ParseStrict`) checks a parser out of an internal `sync.Pool`, re-applies the pool's configured defaults, runs the parse, and returns the parser. Re-applying defaults on checkout is what makes the pool safe: a logger or cancellation flag left on a parser by one request is scrubbed before the next request sees it. Configure pool-wide defaults with the `WithParserPool…` options: `WithParserPoolLogger`, `WithParserPoolTimeoutMicros`, `WithParserPoolIncludedRanges`, `WithParserPoolGLRTrace`, and `WithParserPoolAmbiguityProfile`.

## Parsing a subset: included ranges

`SetIncludedRanges([]Range)` restricts a parse to specific byte ranges of the source; everything outside is skipped. This is the primitive under [language injection](/docs/language-injection) — parsing the SQL inside a Go string, or the JS inside an HTML `<script>` — and you rarely call it directly. `SetIncludedUTF16Ranges` and `SetIncludedUTF16ByteRanges` take the same ranges in UTF-16 code units or bytes, for trees produced by a UTF-16 parse.

## Next steps

- [Incremental Parsing](/docs/incremental-parsing) — reuse a tree across edits instead of re-parsing from scratch.
- [Language Injection](/docs/language-injection) — the high-level API over included ranges.
- [Languages](/docs/languages) — loading languages, and `ParserPool` in a fleet context.
