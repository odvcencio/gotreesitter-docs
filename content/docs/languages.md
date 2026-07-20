---
title: Languages
description: The grammars package in depth — the LangEntry registry, loading languages dynamically, and the full 206-grammar catalog.
nav_group: Languages
order: 1
---

[Getting Started](/docs/getting-started) shows the shortest path to a language: call
`grammars.GoLanguage()`, hand it to `gts.NewParser`, and parse. This page is the deep reference
behind that call — the registry those `XLanguage()` functions live in, what a
`*gotreesitter.Language` actually is, how to load one you did not compile in, and the full list of
the 206 grammars gotreesitter ships.

## Loading a built-in language

Every embedded grammar has a matching `XLanguage() *gotreesitter.Language` function in the
`grammars` package — `GoLanguage()`, `PythonLanguage()`, `RustLanguage()`, and so on for all 206:

```go
import (
	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

lang := grammars.GoLanguage()
parser := gts.NewParser(lang)

tree, err := parser.Parse([]byte("package main\n\nfunc main() {}\n"))
if err != nil {
	panic(err)
}
fmt.Println(tree.RootNode().Type(lang)) // "source_file"
```

The function name does not always spell the grammar name literally: the `c_sharp` grammar's
loader is `CSharpLanguage()`, following Go naming instead of a literal mirror. If you are not sure
of the exact identifier, do not guess — resolve a `LangEntry` from the registry (next section) and
call its `Language` field instead of hardcoding a function name.

Each `XLanguage()` call is cheap after the first: the loader decompresses the blob once and caches
it, so calling `grammars.GoLanguage()` from multiple goroutines or in a loop does not repeat the
work.

## The registry: `LangEntry` and `AllLanguages`

The `grammars` package keeps every built-in grammar in an internal registry, exposed read-only
through `grammars.AllLanguages() []LangEntry`. Calling it is metadata-only — it does **not**
decompress any blob or build any parse table, so it is safe to call on every request, not just at
startup:

```go
entries := grammars.AllLanguages()
fmt.Println(len(entries)) // 206

for _, e := range entries {
	fmt.Println(e.Name, e.Extensions)
}
```

`LangEntry` (`grammars/registry.go`) is the full record behind each name:

| Field | Type | What it's for |
|---|---|---|
| `Name` | `string` | Canonical grammar name — `"go"`, `"c_sharp"`, `"embedded_template"`. What `AllLanguages`, `Register`, and file detection key on. |
| `Extensions` | `[]string` | File suffixes with the leading dot, e.g. `[".go"]` or `[".js", ".mjs", ".cjs"]` for JavaScript. Feeds `DetectLanguage`. |
| `Shebangs` | `[]string` | Exact shebang-line prefixes to match. Unset on every built-in today — shebang detection instead runs off an internal linguist-interpreter table (`#!/usr/bin/env python3` → `python`), which `DetectLanguageByShebang` also consults. The field is for a language that needs an exact, non-table match. |
| `Language` | `func() *gotreesitter.Language` | The lazy loader: first call decodes the grammar, later calls return the cached result. This is what an `XLanguage()` function like `GoLanguage()` actually is. |
| `GrammarSource` | `GrammarSource` | How the grammar was produced: `"ts2go_blob"` (extracted from upstream C `parser.c`, 203 of 206), `"grammargen_blob"` (built by gotreesitter's own pipeline, shipped as a blob — `go`, `regex`, `swift`), or `"grammargen"` (generated at runtime with no embedded blob — how out-of-tree `RegisterExtension` callers typically register). |
| `HighlightQuery` | `string` | The tree-sitter `highlights.scm` text, for syntax-highlighting integrations. |
| `InheritHighlights` | `string` | Parent language whose highlight query gets prepended to this one's (child overrides win). Unset on every built-in; meant for an extension language composing with an existing grammar. |
| `TagsQuery` | `string` | A `tags.scm` query for symbol extraction, when explicitly authored. No built-in ships one explicitly — call `grammars.ResolveTagsQuery(entry)` instead of reading the field directly; it infers a query from the grammar's symbols and caches the result. |
| `TokenSourceFactory` | `func(src []byte, lang *gotreesitter.Language) gotreesitter.TokenSource` | Non-nil only for languages with a hand-written Go token source instead of the DFA lexer (default build: `c`, `cpp`, `java`, `json`, `authzed`). Pass its result to `Parser.ParseWithTokenSource`. |
| `Quality` | `ParseQuality` | Reserved for a `full` / `partial` / `none` classification. Every registered entry has it unset (`""`) today — nothing populates it yet, despite the field's doc comment. For a real classification, call `grammars.AuditParseSupport()` instead. |

`AuditParseSupport() []ParseSupport` answers *how* a language will actually parse — DFA lexer,
hand-written token source, or blocked on a missing external scanner — not just that it is
registered. It loads each grammar (so, unlike `AllLanguages`, it is not free) and reports a
`Backend` (`dfa`, `dfa-partial`, `token_source`, `unsupported`) plus a `Reason` for each.

## Finding a language for a file

`DetectLanguage(filename) *LangEntry` — covered in [Getting Started](/docs/getting-started) — is
the function you will use most: it checks exact filenames first (`Dockerfile`, `.bashrc`), then
registered extensions longest-suffix-first, then a large table of extended extensions. Three more
functions round out the toolkit for cases `DetectLanguage` does not cover:

```go
entry := grammars.DetectLanguageByName("golang")           // linguist alias -> "go"
entry  = grammars.DetectLanguageByShebang("#!/usr/bin/env python3\n") // -> "python"
name  := grammars.DisplayName(entry)                        // "Go" / "Python", not "go" / "python"
```

`DetectLanguageByName` accepts linguist's canonical names and common aliases — `"C++"`, `"cpp"`,
`"Go"`, `"golang"`, `"F#"`, `"fsharp"` all resolve — as well as any of gotreesitter's own grammar
names, and (for extensions registered through `RegisterExtension`) their markdown fence aliases.
`DisplayName` runs the inverse direction for UI: it returns the human title for a grammar name,
falling back to title-casing the name itself when no curated entry exists.

## Loading a language dynamically

Everything above assumes the grammar you want is already compiled into your binary. When it is
not — you are building a plugin system, serving grammars to WASM on demand, or shipping a grammar
your own module generated — load a blob at runtime instead:

```go
blob := grammars.BlobByName("python")       // raw gzip+gob bytes; nil if not embedded
lang, err := grammars.LoadLanguage("python", blob)
if err != nil {
	panic(err)
}
tree, err := gts.NewParser(lang).Parse([]byte("x = 1\n"))
```

`grammars.BlobByName(name string) []byte` returns the same compressed bytes the registry decodes
internally. This is useful for shipping a grammar to a browser-side WASM module or another
process, since every built-in today is blob-backed (`ts2go_blob` or `grammargen_blob`; no
built-in uses the no-blob `grammargen` source).

There are two `LoadLanguage` functions, and the difference matters:

- **`gotreesitter.LoadLanguage(data []byte) (*Language, error)`** (root package) only
  deserializes the blob. It is the only function you need to load a pre-compiled grammar, with no
  other dependency, no `grammargen` import, and no registry.
- **`grammars.LoadLanguage(name string, data []byte) (*gotreesitter.Language, error)`** runs the
  same deserialization, then attaches any external scanner and external-lex-state table
  registered under `name`. Use this one whenever the language has an external scanner — see
  [External Scanners](/docs/external-scanners) — or you will get a `Language` that silently
  cannot recognize its context-sensitive tokens.

Both run the same post-load repair pass (`InferGeneratedRepeatAuxMetadata`); only the `grammars`
loader also attaches scanners. Pick one door and load through it consistently.

Writing and shipping your own grammar this way — with the pipeline that produces the blob in the
first place — is the subject of [Authoring Languages](/docs/authoring-languages).

## Why `Type`, `SExpr`, and field lookups take a `*Language`

[Syntax Trees and Nodes](/docs/syntax-trees-and-nodes) covers the `Node` API in full. Here is the
short version of *why* four of its methods — `Type(lang)`, `SExpr(lang)`,
`ChildByFieldName(name, lang)`, `FieldNameForChild(i, lang)` — need a language argument, a
registry detail worth knowing here.

A `Node` never stores its type or field names as strings. It stores a `Symbol` — a `uint16` — and,
per child, a `FieldID`, also a `uint16`. The strings live once, in the `*Language` that produced
them: `Language.SymbolNames []string` and `Language.FieldNames []string`, indexed by those IDs.
`Type(lang)` resolves to exactly `lang.SymbolNames[node.Symbol()]`, unescaped for display. This is
not an arbitrary API choice: `Node` is a fixed 104-byte struct with a size ratchet enforced by a
test (`TestNodeLayoutSizeBudget` in `tree_test.go`), and a syntax tree can hold millions of nodes.
An 8-byte `*Language` pointer on every single one would be pure overhead for information the tree
already carries once, on `Tree.Language()`. The header has fallen from 152 bytes through several
memory releases, but millions of pointer-rich Go nodes still cost more than C's compact subtrees;
the [Performance](/docs/performance) page records the current memory receipts.

In practice you rarely thread `lang` by hand past the initial parse: `tree.Language()` returns the
same `*Language` the tree was parsed with, so `root.Type(tree.Language())` works from any node
descended from that tree, with no need to keep your own reference around.

## Recovery election isn't a `LangEntry` field — check the `Language`

[Recovery and Correctness](/docs/recovery-and-correctness) covers gotreesitter's C-faithful
recovery election model in full: a language only gets the byte-exact recovery path once it is
both capability-checked and explicitly certified. That status is **not** exposed on `LangEntry` —
there is no `Elected bool` you can read off `AllLanguages()`. It lives on the loaded `*Language`
itself, as `CRecoveryCostCompetitionCapable` and `CRecoveryCostCompetitionEnabledByDefault`:

```go
entry := grammars.DetectLanguageByName("rust")
lang := entry.Language()
fmt.Println(lang.CRecoveryCostCompetitionEnabledByDefault) // true: rust is elected
```

Checking this across every registered language (`entry.Language().CRecoveryCostCompetitionEnabledByDefault`
for each of the 206) is how this page's own catalog numbers below were verified: **124 of 206**
languages are certified today, matching the project's public "\~124 elected" figure exactly. The
other 82 use the resync recovery path described on the Recovery and Correctness page — safe and
non-hanging, just not certified byte-identical to C on damaged input.

## The full catalog

This list comes from calling `grammars.AllLanguages()` and printing every `Name`, sorted — not
hand-typed, so it stays honest as the set grows. Here is the scan behind it:

| | |
|---|---|
| Total registered languages | **206** |
| `ts2go_blob` (extracted from upstream `parser.c`) | 203 |
| `grammargen_blob` (built by gotreesitter's own pipeline) | 3 (`go`, `regex`, `swift`) |
| File extensions mapped across all entries | 273 |
| Entries with a hand-written `TokenSourceFactory` | 5 |
| Elected into C-faithful recovery (measured, see above) | 124 |

```langlist
ada                csv                forth              json               pem                starlark
agda               cuda               fortran            json5              perl               svelte
angular            cue                fsharp             jsonnet            php                swift
apex               cylc               gdscript           julia              pkl                tablegen
arduino            d                  git_config         just               powershell         tcl
asm                dart               git_rebase         kconfig            prisma             teal
astro              desktop            gitattributes      kdl                prolog             templ
authzed            devicetree         gitcommit          kotlin             promql             textproto
awk                dhall              gitignore          ledger             properties         thrift
bash               diff               gleam              less               proto              tlaplus
bass               disassembly        glsl               linkerscript       pug                tmux
beancount          djot               gn                 liquid             puppet             todotxt
bibtex             dockerfile         go                 llvm               purescript         toml
bicep              dot                godot_resource     lua                python             tsx
bitbake            doxygen            gomod              luau               ql                 turtle
blade              dtd                graphql            make               r                  twig
brightscript       earthfile          groovy             markdown           racket             typescript
c                  ebnf               hack               markdown_inline    regex              typst
c_sharp            editorconfig       hare               matlab             rego               uxntal
caddy              eds                haskell            mermaid            requirements       v
cairo              eex                haxe               meson              rescript           verilog
capnp              elisp              hcl                mojo               robot              vhdl
chatito            elixir             heex               move               ron                vimdoc
circom             elm                hlsl               nginx              rst                vue
clojure            elsa               html               nickel             ruby               wat
cmake              embedded_template  http               nim                rust               wgsl
cobol              enforce            hurl               ninja              scala              wolfram
comment            erlang             hyprlang           nix                scheme             xml
commonlisp         facility           ini                norg               scss               yaml
cooklang           faust              janet              nushell            smithy             yuck
corn               fennel             java               objc               solidity           zig
cpon               fidl               javascript         ocaml              sparql
cpp                firrtl             jinja2             odin               sql
crystal            fish               jq                 org                squirrel
css                foam               jsdoc              pascal             ssh_config
```

That list holds grammar *names* — the registry key and the argument to `DetectLanguageByName`.
Several map to more than one extension (`javascript` covers `.js`, `.mjs`, `.cjs`); a few read
differently from the file type's common name (`c_sharp` for `.cs`, `embedded_template` for ERB).
When in doubt, resolve by filename with `DetectLanguage` instead of guessing the registry name
from the extension.

## Next steps

- [Authoring Languages](/docs/authoring-languages) — bring a grammar that isn't in the 206, without
  forking gotreesitter.
- [External Scanners](/docs/external-scanners) — when a grammar needs hand-written Go lexing, and
  how to port one.
- [Recovery and Correctness](/docs/recovery-and-correctness) — the full election model behind the
  124-language figure above.
- [Syntax Trees and Nodes](/docs/syntax-trees-and-nodes) — the complete `Node` API.
