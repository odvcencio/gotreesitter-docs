---
title: Playground
description: A live, in-browser syntax-tree explorer running gotreesitter's actual production parser via WASM — coming soon.
nav_group: Project
order: 1
---

The playground will be a live, in-browser syntax-tree explorer: pick a language, type or paste
code, and watch the tree gotreesitter builds update as you type. No install, and — in the target
design — no server round trip for the parse itself.

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

## What already exists

This isn't just a plan — the groundwork is in the repository today. `wasm/runtime/main.go` is a
`GOOS=js GOARCH=wasm` build that exposes `parse`, `highlight`, and `loadBlob` as JavaScript globals
via `syscall/js`, and `wasm/loader.js` is a small loader that fetches and instantiates the compiled
module in a browser tab. A second entry point, `wasm/grammargen/main.go`, does the same for
importing a `grammar.json` and generating a parser on the fly, entirely client-side. Together
they're a working proof that the approach compiles and runs — what's missing is the actual site
around it: an editor pane, a language picker across the 206 embedded grammars, and a tree view wired
up to that bridge.

## Coming soon

The interactive playground itself — the page you'd click into and start typing — isn't built yet.
This page will turn into that experience in a later pass. Until then, the fastest way to see a real
syntax tree is still the Go side: [Getting Started](/docs/getting-started) has a short program that
parses a file and prints its tree, and [Introduction](/docs/introduction) covers what gotreesitter
checks that tree against.
