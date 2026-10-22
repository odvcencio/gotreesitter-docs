---
title: Playground
description: A live, in-browser syntax-tree explorer running gotreesitter's production parser via a GoSX-managed WebAssembly engine.
nav_group: Project
order: 1
---

[The playground](/playground) is a live, in-browser syntax-tree explorer: pick any of the 206
languages and watch the tree gotreesitter builds update as you type. There is no install step,
and no server round trip for parsing or query execution.

That last part is the distinctive feature. gotreesitter is pure Go: no CGo, no C toolchain, and
nothing to cross-compile separately per target. The same `go build` that produces a native binary
also cross-compiles to `GOOS=js GOARCH=wasm` (the browser WASM target) and to `wasip1`, with the
standard Go toolchain and no extra steps. Pointed at the browser target, that means the playground
can run the *actual production parser* — the same GLR engine and grammar tables described in
[Architecture](/docs/architecture) — client-side, rather than a simplified reimplementation or a
parse endpoint on a server somewhere. That is the real contrast with upstream tree-sitter's own
web playground, which compiles the C runtime and each grammar's C parser to WASM through
Emscripten — a separate, heavier build step that produces one `.wasm` artifact per grammar.
gotreesitter's parser and its 206 grammars are just Go code and data, so one build targets all of
them at once.

## How it works

The page ships the engine-only runtime — the parser, GLR core, recovery, query engine, scanner
adapters, and GoSX engine registration — as one standard-Go WASM module. Grammar tables are not
baked in. The browser engine first fetches a small 206-language index; picking a language then
fetches that content-hashed compiled grammar blob and attaches gotreesitter's registered scanner
support locally. Loaded languages stay cached in the tab, and every keystroke after that parses
with no network request.

Only immutable program data crosses the network: the GoSX runtime, the parser engine, the grammar
index, and selected grammar blobs. Source and query text never enter a request unless you copy a share link. The production
browser gate types a private marker, verifies it parses locally, switches to a second language to
prove lazy blob loading, and rejects any non-GET request or URL containing the marker.

## Deep links

`/playground?lang=<grammar>` opens a grammar with its curated starter. Add `&sample=add` to
open the three-line `add` snippet from the home page, for example
[`/playground?lang=python&sample=add`](/playground?lang=python&sample=add). The `add` sample
exists for Go, Python, Rust, TypeScript, JSON and HTML. `&sample=greet`, a missing `sample`, or an
unknown name opens the curated starter. A language without a curated starter opens a comment
probe that parses in that grammar.

## Share links

`Copy share link` puts a link on your clipboard that opens the same grammar, source and query:
`/playground?lang=<grammar>&s=<payload>` with an optional `&q=<payload>` for the query. Each
payload is `1.` followed by the unpadded base64url form of the raw DEFLATE stream of the UTF-8
text. The `1.` prefix lets a later version change the encoding without breaking old links.

Limits: the decoded source may be up to 65,536 bytes and the decoded query up to 8,192 bytes, and
each encoded payload may be up to 7,000 characters so the whole URL fits proxy request-line limits.
The button reports when the editor is too large to share. A link that is damaged or over the limit
opens the playground with the sample and a short notice; it never returns an error page.

Creating a link makes no request. Opening a link sends the source to the server, because it is in
the URL, so share only text you are willing to put in a link.

[Open the playground](/playground), or read [Architecture](/docs/architecture) for what that
engine is actually doing under your keystrokes.
