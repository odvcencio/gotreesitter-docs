---
title: External Scanners
description: When a tree-sitter grammar needs hand-written Go lexing, and the porting contract for writing one that matches C tree-sitter byte-for-byte.
nav_group: Languages
order: 4
---

External scanners are the escape hatch tree-sitter grammars use when a token cannot be recognized
by a regular lexer: the grammar's `externals` array names the tokens, and hand-written scanner code
recognizes them with full context. In upstream tree-sitter that code is `src/scanner.c`; in
gotreesitter it's a Go type implementing `gotreesitter.ExternalScanner`, attached to
`Language.ExternalScanner`.

This page is in two parts: first, deciding whether you need a scanner at all (most grammars that
think they do, don't); second, the Go porting contract, including behavioral requirements this
project learned the hard way while chasing byte-exact parity with C tree-sitter.

See [Authoring Languages](/docs/authoring-languages) for the grammar → blob → runtime pipeline
this fits into.

## Part 1 — the decision

### ✗ You do NOT need a scanner for

- **Keyword vs. identifier.** Declare a `word` token (`"word": "identifier"` in grammar.json,
  `g.SetWord("identifier")` in the DSL) and write keywords as plain strings. Keyword extraction
  handles the overlap.
- **Precedence and associativity.** `prec`, `prec.left`, `prec.right`, `prec.dynamic` (DSL: `Prec`,
  `PrecLeft`, `PrecRight`, `PrecDynamic`) resolve expression-grammar ambiguity in the tables.
  Dangling-else, cast-vs-call, operator towers — all in-grammar.
- **Comments and whitespace.** They're `extras`. gotreesitter compiles a whitespace pattern extra
  into DFA skip transitions and shifts visible extras (like `comment`) anywhere.
- **Plain strings and numbers.** `token(seq('"', /[^"]*/, '"'))` and friends. A string with a fixed
  delimiter and escape rules is regular.
- **C-style preprocessors.** tree-sitter-c parses `#if`/`#define`/`#include` entirely in-grammar
  (directives are line-oriented rules; `\\\n` splices are part of token patterns). "It has a
  preprocessor" is not, by itself, a scanner reason — Pawn only needed scanner help for two
  *recovery* cases, below.

If your grammar is in one of these buckets, stop here: every scanner you don't write is corpus
parity you don't have to re-verify.

### ✓ You DO need a scanner for

Context-sensitive or unbounded-lookahead lexing — when what the next token *is* depends on
arbitrary distance or on state the LR automaton cannot carry:

- **Indentation blocks** (Python, YAML): a stack of indent widths compared on each newline. Needs
  serialized mutable state.
- **Heredocs** (bash, Perl, Ruby): the closing delimiter is chosen at open time and matched
  arbitrarily later.
- **Nested string interpolation** (Elixir, JavaScript templates): string content tokens whose
  validity depends on interpolation depth.
- **Delimiter-matched raw strings** (Rust `r###"..."###`, C# raw strings): count at open, match the
  same count at close.
- **Newline significance / optional semicolons**: Go's automatic-semicolon rule and Pawn's "lax
  mode" (statements may end at the line break) are the canonical cases. The terminator token exists
  only when the *next* line does not continue the statement — pure lookahead-past-the-token
  territory.
- **Contextual token disambiguation**: Pawn's `public OnFoo(<Bar>)` callback signature opener `<`
  versus relational `i < 10`. Same character, different token, decided by what follows.
- **Zero-width / sentinel tokens**: tokens that consume nothing but tell the parser "a block ended
  here" (implicit end tags in HTML, dedents, the terminators below).
- **Bounded recovery for messy constructs**: consuming a known-unparseable region as one opaque
  token so the rest of the file parses. Pawn's `#define` header recovery is this.

### Worked case study: Pawn's five externals

pawnkit's tree-sitter-pawn declares exactly five external tokens (`grammar.js` `externals`), with a
deliberately narrow, **stateless** `scanner.c` — a good first port because there is no
serialization to get wrong:

| Token | What it does | Why the lexer can't |
|---|---|---|
| `_callback_signature_start` | Recognizes `<` as the opener of a callback signature. Consumes `<`, marks the token end there, then *looks ahead without consuming*: optional whitespace, then `>`, `_`, `%ident`, or an identifier, then `>`, and — when there was content — requires that the line does not immediately end (EOF, `\n`, `;`, or a comment running to end-of-line all reject). | Distinguishing signature `<` from relational `<` needs lookahead through an identifier, whitespace, comments, and the following construct. |
| `_statement_line_terminator` | Pawn's lax mode: a statement ends at a newline. Skips horizontal whitespace, comments, and `\`-newline line splices first; at EOF, terminates; at `\n`, consumes it, marks the end, then keeps scanning forward — if the next non-trivia character is `{` or a continuation character (`(`, `[`, `.`, `,`, `?`, `:`, operators…), it rejects, because the statement continues. | The token's existence depends on the first meaningful character of the *next* line. Classic unbounded lookahead. |
| `_directive_line_terminator` | Ends a preprocessor directive: skip spaces/tabs/CR; EOF terminates; `\n` is consumed and the end marked after it. | Line-oriented terminator shared by all directives; kept external so it composes with the two recovery tokens. |
| `_unsupported_define_header` | Bounded recovery: when a `#define` header has a shape the grammar doesn't model, consume it byte-by-byte up to whitespace, `/`, or end-of-line, marking the end as it goes, and hand the parser one opaque token. | Recovery: without it, an exotic `#define` shatters into a cascade of ERROR nodes. |
| `_unsupported_macro_parameter_list` | Same idea for macro parameter lists: consume a balanced-paren region (tracking nesting depth, and quoted strings with escapes) until the matching close paren or end-of-line, and only succeed when it actually saw something unsupported. | Balanced-delimiter matching with embedded strings is beyond regular lexing; doing it in the parser would poison the grammar with junk rules. |

Note the shape: two disambiguation/terminator tokens driven by lookahead, one line-terminator, two
recovery tokens. Zero bytes of persistent state — the C `create`/`serialize` are no-ops. Keep your
externals list this honest.

## Part 2 — porting a scanner to Go

### The interface

From `language.go`:

```go
type ExternalScanner interface {
	Create() any
	Destroy(payload any)
	Serialize(payload any, buf []byte) int
	Deserialize(payload any, buf []byte)
	Scan(payload any, lexer *ExternalLexer, validSymbols []bool) bool
}
```

Lifecycle, as the runtime actually drives it (`parser_dfa_token_source.go`):

- `Create` is called when a token source is initialized for a parse; the returned payload is
  passed to every other method. Use a pointer to a concrete struct; scanners conventionally panic
  on a wrong payload type.
- `Destroy` is called when the token source is closed/reset.
- `Serialize(payload, buf) int` writes scanner state into `buf` and returns the byte count. The
  runtime hands you a 4096-byte buffer (`externalScannerSerializationBufferSize`) — larger than
  C's 1024 — and snapshots state around retries and, when checkpoints are enabled, **after
  external tokens**, so keep it allocation-free and cheap. `Deserialize` restores from a snapshot
  (which may be empty ⇒ reset to initial state).
- `Scan` returns true if it recognized a token. On true, the runtime reads the result from the
  lexer (symbol + span). On false, position effects are discarded (state effects are not — see
  `FailurePreservingExternalScanner` below).

### The dual numbering contract (differs from C!)

Two different number spaces appear in `Scan`, and they are not the same space — unlike C, where
`result_symbol` takes the same enum used to index `valid_symbols`:

- `validSymbols[i]` is indexed by **external token index**: `i` is the position in the grammar's
  `externals` array, which is also the index into `Language.ExternalSymbols` (grammargen preserves
  this order — `registerExternalSymbols` in `grammargen/normalize.go` walks `Grammar.Externals` in
  order).
- `ExternalLexer.SetResultSymbol(sym Symbol)` takes the **language symbol ID** — i.e.
  `lang.ExternalSymbols[i]`, not `i`.

Every scanner in this repo therefore keeps two constant sets (see `grammars/svelte_scanner.go` for
the pattern): token indexes for gating, symbol IDs for results. Resolve the symbols from the
`Language` at construction time rather than hard-coding them.

### The lexer API

From `external_lexer.go`, with C equivalents:

| Go | C | Semantics |
|---|---|---|
| `Lookahead() rune` | `lexer->lookahead` | Current rune; `0` at EOF. |
| `Advance(skip bool)` | `lexer->advance(lexer, skip)` | Consume one rune. `skip=true` moves the token *start* forward (whitespace exclusion) and — exactly like C — does **not** move the token end; `MarkEnd` is the only way to set the end. |
| `MarkEnd()` | `lexer->mark_end(lexer)` | Set token end = current position. |
| `SetResultSymbol(sym Symbol)` | `lexer->result_symbol = ...` | See numbering contract above. |
| `Column() uint32` | `lexer->get_column(lexer)` | 0-based column at the cursor. |
| `HasPreviousBytes(text string) bool` | (no C equivalent) | True if the bytes immediately before the cursor equal `text`; used to guard content tokens when merged parser states expose them too broadly. |
| `AdvanceSpaces(skip bool) int`, `AdvanceUntilNewline(skip bool) int` | (helpers) | Bulk equivalents of repeated `Advance` for ASCII-space runs / to-end-of-line runs. |

Span rules you must not reinvent (they mirror C's `ts_lexer` exactly, and the comments in
`external_lexer.go` document why):

- If `Scan` returns true and you never called `MarkEnd`, the end defaults to the current cursor —
  including cursor movement from `Advance(true)`.
- If you `MarkEnd` and then `Advance(true)` past the mark, the token becomes **zero-width at the
  mark** and the parser re-positions there, so the skipped bytes are lexed again next call. This is
  how YAML-style and terminator tokens work; it's deliberate, not a bug.

### A faithful Go port of Pawn's scanner (condensed)

```go
package pawn

import gts "github.com/odvcencio/gotreesitter"

// External token indexes: order of grammar.json "externals".
const (
	tokCallbackSignatureStart = iota
	tokStatementLineTerminator
	tokDirectiveLineTerminator
	tokUnsupportedDefineHeader
	tokUnsupportedMacroParameterList
	tokCount
)

// PawnScanner is stateless, like upstream scanner.c.
type PawnScanner struct {
	syms [tokCount]gts.Symbol // external index -> language symbol ID
}

func NewPawnScanner(lang *gts.Language) *PawnScanner {
	s := &PawnScanner{}
	copy(s.syms[:], lang.ExternalSymbols)
	return s
}

func (s *PawnScanner) Create() any                           { return nil }
func (s *PawnScanner) Destroy(any)                           {}
func (s *PawnScanner) Serialize(payload any, buf []byte) int { return 0 }
func (s *PawnScanner) Deserialize(payload any, buf []byte)   {}

// Stateless scanners can safely opt into both fast paths.
func (s *PawnScanner) SupportsIncrementalReuse() bool    { return true }
func (s *PawnScanner) PreservesStateOnScanFailure() bool { return true }

func (s *PawnScanner) Scan(payload any, lx *gts.ExternalLexer, valid []bool) bool {
	if valid[tokCallbackSignatureStart] && scanCallbackSignatureStart(lx) {
		lx.SetResultSymbol(s.syms[tokCallbackSignatureStart])
		return true
	}
	if valid[tokStatementLineTerminator] && scanStatementLineTerminator(lx) {
		lx.SetResultSymbol(s.syms[tokStatementLineTerminator])
		return true
	}
	if valid[tokDirectiveLineTerminator] && scanDirectiveLineTerminator(lx) {
		lx.SetResultSymbol(s.syms[tokDirectiveLineTerminator])
		return true
	}
	// ... the two recovery tokens follow the same shape ...
	return false
}

// scanDirectiveLineTerminator: skip spaces/tabs/CR; EOF terminates; a
// newline is consumed (as skip) and the end marked after it — a zero-width
// terminator, exactly like the C original.
func scanDirectiveLineTerminator(lx *gts.ExternalLexer) bool {
	for lx.Lookahead() == ' ' || lx.Lookahead() == '\t' || lx.Lookahead() == '\r' {
		lx.Advance(true)
	}
	if lx.Lookahead() == 0 { // EOF: see contract (b) below
		lx.MarkEnd()
		return true
	}
	if lx.Lookahead() != '\n' {
		return false
	}
	lx.Advance(true)
	lx.MarkEnd()
	return true
}

// scanCallbackSignatureStart shows the mark-then-look-ahead idiom: the token
// is exactly "<"; everything after the MarkEnd is validation only.
func scanCallbackSignatureStart(lx *gts.ExternalLexer) bool {
	if lx.Lookahead() != '<' {
		return false
	}
	lx.Advance(false)
	lx.MarkEnd()
	// ... whitespace-skip, then accept ">", "_", "%ident", or an identifier,
	// then ">", then reject if the line effectively ends (EOF/'\n'/';'/line
	// comment) — consuming freely, because the mark already froze the span.
	// Port the C control flow 1:1; do not "simplify" the reject conditions.
	...
	return true
}
```

Gate every branch on `valid[...]`. The runtime computes `validSymbols` from the grammar tables —
for grammargen-built languages via the `Language.ExternalLexStates` table (built automatically when
the grammar has externals; it mirrors C's `ts_external_scanner_states`), unioned across all live
GLR stacks when the parse has forked (`SetGLRStates`). Returning a symbol that isn't valid in the
current state is undefined-behavior territory: at best pruned, at worst an error cascade.

### Wiring the scanner to the Language

Simplest, registry-free — assign the public field:

```go
lang, err := gts.LoadLanguage(pawnBlob)
...
lang.ExternalScanner = NewPawnScanner(lang)
```

If you distribute through the `grammars` registry (`grammars.RegisterExternalScanner(name, s)` +
`grammars.RegisterExternalLexStates(name, states)`, then `grammars.LoadLanguage(name, blob)` /
`grammars.AttachLanguageSupport`), one caveat verified against `grammars/embedded_loader.go`: for a
language whose name has **no embedded reference blob in this repo**, the attach path
(`AdaptScannerForLanguage`) can only bind your scanner if it implements

```go
ExternalScannerForLanguage(lang *gts.Language) gts.ExternalScanner
```

(the `languageBoundExternalScanner` hook). Without it, the adapter tries to load the in-repo
reference blob to remap symbol IDs — and for an out-of-tree name that lookup panics. Implement the
hook (return `NewPawnScanner(lang)`) or skip the registry and assign the field directly. If you
need to move a scanner between two Languages with different symbol numbering, the public remapper
is `gotreesitter.AdaptExternalScannerByExternalOrder(sourceLang, targetLang) (ExternalScanner,
bool)`. See [Languages](/docs/languages) for the rest of the registry surface.

### Optional capability interfaces

- `IncrementalReuseExternalScanner` (`SupportsIncrementalReuse() bool`): declare true only if
  reusing subtrees across edits is safe for your state. Stateless scanners: yes. Python-style
  indent stacks deliberately leave this unimplemented so edits force conservative reparse.
- `FailurePreservingExternalScanner` (`PreservesStateOnScanFailure() bool`): declare true if `Scan`
  returning false never mutated the payload — lets the runtime skip defensive state snapshots on
  the hot path.

## Hard-learned behavioral contracts

These four came out of this project's C-parity work. They apply mainly when you implement a
**full custom `TokenSource`** (`parser_api.go`: `Next() Token`, returning a zero-`Symbol` token at
EOF) instead of, or in addition to, an external scanner — but (a) and (b) bite scanner authors too.

**(a) Emit extras; never skip silently.** If the grammar declares whitespace as an extra token,
your token source must *emit* it as that extra symbol, not swallow it. Real incident: a
hand-written token source skipped horizontal whitespace instead of emitting the grammar's
`_whitespace` extra; C shifts whitespace extras, which advances the parse position, so when error
recovery re-lexed, the Go anchor was one byte behind C's and the ERROR spans diverged. The fix and
its rationale are preserved as a comment in `grammars/authzed_lexer.go` ("emit it the same way
instead of silently skipping, so error recovery re-lexes at the true content byte"). For external
scanners the analogue: use `Advance(skip=true)` only for bytes the grammar really treats as
skippable in that context, and remember skip never moves the marked end.

**(b) EOF must mirror C: no accept at EOF without a matched token.** At end of input with nothing
matched, return the zero-`Symbol` EOF token at the EOF position (`lexer.go` does exactly this) — do
not promote a partial match that never reached an accepting state, and do not fabricate a
terminator the grammar didn't ask for. This repo shipped a fix titled "mirror C tree-sitter
behavior for EOF without accept" because getting it wrong flips end-of-file reductions and changes
the last node of every tree. External scanners: `Lookahead() == 0` is EOF; returning true there is
only correct for genuine zero-width EOF-terminator tokens (Pawn's terminators, dedents), with
`MarkEnd` placed deliberately.

**(c) Error-mode lexing.** C's `ts_parser__lex` re-lexes at the recovery frontier with the most
permissive lex mode — `LexModes[0]`, the ERROR_STATE mode — and the faithful C recovery port
expects the same: after `SetParserState(0)`, tokens should carry error-mode identity. The built-in
DFA token source honors this. The runtime discovers the capability through the
`errorModeLexingTokenSource` interface in `parser_api.go` (`lexesErrorModeAtErrorState() bool`) —
but note honestly: that method name is unexported, so **an out-of-tree token source cannot
currently declare the capability** even if it implements the behavior (Go's unexported interface
methods are package-scoped). What happens instead (`parser_recover_c.go`,
`cRecoverCustomSourceEligibleFor`): if your source supports `SkipToByte`, the grammar has usable
lex tables, **and it has no external scanner/symbols**, the engine substitutes its own DFA in error
mode and resyncs your source afterwards; otherwise recovery decisions can diverge from C on
error-bearing inputs. Until the marker is exported, implement `SetParserState(0)` ⇒
most-permissive-lexing anyway (it's the correct behavior), support `SkipToByte`, and test error
inputs against the C oracle rather than assuming. See
[Recovery and Correctness](/docs/recovery-and-correctness) for the full election model this feeds
into.

**(d) Parser-state plumbing for TokenSource implementers.** The parser feeds context through
optional, structurally-matched methods (all exported names, so out-of-tree types satisfy them):

- `SetParserState(state StateID)` — called before lexing each token with the primary live stack's
  state; state selects the lex mode and, for scanners, the `ExternalLexStates` row. State 0 is the
  error state (see (c)).
- `SetGLRStates(states []StateID)` — when multiple GLR stacks are live, the full set of stack-top
  states; compute external-token validity as the **union** across them (this is exactly what the
  built-in source does), then let the parser prune. Cleared (or single-state) when the fork
  collapses.
- `SkipToByte(offset uint32) Token` / `SkipToByteWithPoint(offset uint32, pt Point) Token` — jump
  to a byte offset and return the first token at or after it. Required for incremental subtree
  reuse (`IncrementalReuseTokenSource`, `SupportsIncrementalReuse() bool`) and used by recovery
  resync. Must be deterministic: skipping to offset N then calling `Next` repeatedly must yield the
  same stream as `Next`-ing from the start past N.
- If the parse uses `Parser.SetIncludedRanges`, your source gets wrapped by an included-range
  filter that forwards `SetParserState`/`SetGLRStates`/`SkipToByte`/error-mode queries to you
  (`included_ranges.go`) — implement the methods on the base source and the wrapper composes for
  free.

## Before you ship a scanner port

- [ ] Run your grammar's full corpus through both the C parser and the Go port and compare
      S-expressions **byte-exact** — not "no errors", exact. tree-sitter-pawn keeps its corpus
      under `test/corpus/`; that's the oracle.
- [ ] Test EOF edges specifically: file ending exactly at your token, ending in whitespace, ending
      mid-construct, empty file, file with only a BOM.
- [ ] Test **error inputs**, not just clean ones — recovery is where (a), (b), and (c) show up, and
      it's the least-tested path in every port.
- [ ] If the scanner has state: `Serialize` → `Deserialize` → `Serialize` must be a fixed point,
      and state must fit 4096 bytes at your worst nesting depth.
- [ ] If any token can be zero-width: confirm the parser makes progress on adversarial inputs (the
      runtime caps consecutive zero-width tokens, but hitting the cap means your validity gating is
      wrong).
- [ ] Gate every `Scan` branch on `validSymbols`; never return a symbol whose index wasn't valid.

## Next steps

- [Authoring Languages](/docs/authoring-languages) — the full grammar → blob → runtime pipeline an
  external scanner plugs into.
- [Languages](/docs/languages) — the `grammars` registry, `LoadLanguage`, and the full catalog.
- [Recovery and Correctness](/docs/recovery-and-correctness) — why error-mode lexing identity
  matters for C-faithful recovery.
