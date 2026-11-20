---
title: Parser Engines and Upgrade Guide
description: Which grammars use the certified C-faithful core and how to compare parser routes before upgrading an editor.
nav_group: Using the Parser
order: 9
---

The current published module and the playground use v0.57.0. This guide describes
the certified defaults on main ahead of v0.58.0.

gotreesitter is a pure-Go tree-sitter runtime. It uses one public parser API for
the original GLR engine and the certified port of the C parser, called ccore.
No C toolchain or additional dependency is needed to use either engine.

## Which grammars use the certified core?

At [main revision `aaa85d2c`](https://github.com/odvcencio/gotreesitter/commit/aaa85d2c9f7816beb5fcb424622aaf15c2cba774),
34 grammars have default certificates ahead of v0.58.0.

| | Certified grammar names |
| --- | --- |
| Programming | C, Clojure, Elixir, Erlang, Groovy, Kotlin, Objective-C, Python, R, Ruby, Rust, Solidity, TypeScript, Zig |
| Data, configuration, and styles | BibTeX, CSS, Desktop Entry, Devicetree, EBNF, Git Config, GraphQL, INI, JSON, Protobuf, SSH config, Textproto, TOML |
| Build and version control | Cap’n Proto, Diff, Dockerfile, Git Commit, Go Modules, Make, Todo.txt |

The [certificate list at that revision](https://github.com/odvcencio/gotreesitter/blob/aaa85d2c9f7816beb5fcb424622aaf15c2cba774/testdata/ccore_certificates.json)
records the exact grammar artifacts. A request also has to use a covered
configuration and scanner. A parse that sets included ranges, a custom token
source, parse work limits or parse-observability hooks uses the existing
engine. Grammars not in the list, such as Go, JavaScript and TSX at that
revision, use their existing routes.

## Upgrade an editor integration

1. Keep using the same `Parser` or registered grammar entry. For an A/B check,
   create a second parser with the same grammar and call
   `SetAdmissionCandidateRoute(false)` on it. The setting applies from that
   parser's next parse; set it before the first parse so each comparison has
   its own tree history. The process-wide default
   (`SetAdmissionCandidateRouteDefault`, `GTS_ADMISSION_CANDIDATE`) does not
   move certified grammars off the core.
2. Pass the exact source for each parse. Record each edit with `Tree.Edit`
   before reparsing. Release each returned handle, including an unchanged
   result that uses the old pointer. [Incremental example](/docs/incremental-parsing)
3. Compare fresh and edited trees on the files and keystrokes your application
   sees. Check syntax errors separately from early stops. If a reader must keep
   an old snapshot while editing, give it its own `Tree.Copy()`.
4. Measure initial parse and edit latency separately. Use the same grammar
   revision, input, edit history, and output work for both implementations.

Graduation checks fresh and edited syntax, scanner behavior, subtree reuse,
tree ownership, memory, and latency. For example, an edit starting from a
malformed tree can take a fresh parse instead of attempting reuse. A certificate
records the tested behavior of one grammar. It does not specify a speed ratio
for other files or configurations. [Graduation gates](/docs/v1#how-one-language-graduates)

See [Performance](/docs/performance#certified-core-comparisons) for the fresh-parse and editor benchmarks.
