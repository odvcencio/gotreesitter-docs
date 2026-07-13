---
title: Playground
description: A live, in-browser syntax-tree explorer running gotreesitter's actual production parser via WASM — with automatic language detection.
nav_group: Project
order: 1
---

[The playground](/playground) is a live, in-browser syntax-tree explorer: pick a language — or
just start typing and let it detect one — and watch the tree gotreesitter builds update as you
type. No install, and no server round trip for parsing or query execution.

That last part is the distinctive bit. gotreesitter is pure Go: no CGo, no C toolchain, nothing to
cross-compile separately per target. The same `go build` that produces a native binary already
cross-compiles to `GOOS=js GOARCH=wasm` (the browser WASM target) and to `wasip1`, with the standard
Go toolchain and no extra steps. Pointed at the browser target, that means the playground can run
the *actual production parser* — the same GLR engine and grammar tables described in
[Architecture](/docs/architecture) — client-side, rather than a simplified reimplementation or a
parse endpoint on a server somewhere. That's the real contrast with upstream tree-sitter's own web
playground, which compiles the C runtime and each grammar's C parser to WASM via Emscripten — a
separate, heavier build step that produces one `.wasm` artifact per grammar. gotreesitter's parser
and its 206 grammars are just Go code and data, so one build targets all of them at once.

## How it works

The page ships the engine-only runtime — the parser, the GLR core, recovery, the query engine,
and the highlighter, built with `GOOS=js GOARCH=wasm` — as a single WASM module (about 3.6 MB
compressed). Grammars are not baked in: picking a language fetches that language's compiled
grammar blob on demand (2 KB for JSON, ~40 KB for JavaScript, ~120 KB for TypeScript) and hands
it to the runtime's `loadBlob`. After that, every keystroke parses locally in your tab; the
status readout shows the real parse time.

Language auto-detection is the parser doing double duty. Obvious signals (a shebang,
`package main`, `#include`) switch instantly, client-side. For everything else, a debounced
request races a bounded source sample against a shortlist of grammars server-side — all 206 live
in one process, so the race is just parse-and-count-error-nodes — and the cleanest tree wins. The
request body is capped at 64 KiB and the detector scores at most the first 4 KiB. Picking a language
manually always overrides detection and avoids sending source to the detector.

[Open the playground](/playground), or read [Architecture](/docs/architecture) for what that
engine is actually doing under your keystrokes.
