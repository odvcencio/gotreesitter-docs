---
title: Getting Started
description: Install gotreesitter, parse your first file, and learn to read the syntax tree it gives back.
nav_group: Introduction
order: 2
---

This page gets you from zero to a parsed syntax tree, then through the handful of `Node` and
`Language` methods you'll reach for constantly: reading a node, walking its children, and telling
named structure apart from literal punctuation.

## Install

```sh
go get github.com/odvcencio/gotreesitter
```

That one command is enough. The parsing engine (`github.com/odvcencio/gotreesitter`) and the 206
embedded grammars (`github.com/odvcencio/gotreesitter/grammars`) live in the same Go module, so
once it's a dependency you can import either package — there's no separate `go get` for the
grammars subpackage.

## Your first parse

A complete program: parse a small Go file and print its syntax tree as an S-expression.

```go
package main

import (
	"fmt"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func main() {
	src := []byte(`package main

func main() {
	println("hello, world")
}
`)

	lang := grammars.GoLanguage()
	p := gts.NewParser(lang)

	tree, err := p.Parse(src)
	if err != nil {
		panic(err)
	}

	root := tree.RootNode()
	fmt.Println(root.SExpr(lang))
}
```

Running it prints:

```text
(source_file (package_clause (package_identifier)) (function_declaration (identifier) (parameter_list) (block (statement_list (expression_statement (call_expression (identifier) (argument_list (interpreted_string_literal (interpreted_string_literal_content)))))))))
```

Walking through it:

- `grammars.GoLanguage()` returns a `*gts.Language` — decoded parse tables for Go. Every one of
  the 206 grammars has a matching `XLanguage()` function in the `grammars` package.
- `gts.NewParser(lang)` builds a `*gts.Parser` bound to that language. A `Parser` is cheap to
  reuse — call `Parse` on it as many times as you like.
- `p.Parse(src)` returns a `*gts.Tree` and an `error`. The error covers setup problems: no
  language attached, a language version incompatible with the runtime, or a hand-authored grammar
  that has no DFA lexer and needs `ParseWithTokenSource` instead. A source file with syntax errors
  in *it* still parses successfully with a `nil` error — the tree records the problem in its nodes
  rather than failing the call. Check `tree.RootNode().HasError()` when you need to know whether
  the source was clean.
- `tree.RootNode()` is where every traversal starts: a `*gts.Node` covering the whole file.
- `root.SExpr(lang)` renders the tree in tree-sitter's S-expression format, handy for debugging
  and golden-file tests. It only prints named nodes — more on what that means below.

## Reading a node

A `*gts.Node` is a position and a type; it doesn't carry the source text itself, so anything that
needs text takes the original `[]byte` as an argument. Reach into the tree from above and look at
the function declaration:

```go
fn := root.NamedChild(1) // function_declaration; NamedChild(0) is the package clause

fmt.Println("type:", fn.Type(lang))
fmt.Println("bytes:", fn.StartByte(), "-", fn.EndByte())
fmt.Println("text:", fn.Text(src))
```

```text
type: function_declaration
bytes: 14 - 54
text: func main() {
	println("hello, world")
}
```

`Type(lang)` needs the language because node types are small integer symbol IDs internally — the
`*gts.Language` is what maps a symbol back to a name like `"function_declaration"`. `StartByte()`
and `EndByte()` give a byte range into the source; `Text(source)` slices that range out of whatever
`[]byte` you pass it (normally the same `src` you parsed).

## Walking children

Every node exposes both its full child list and a named-only view:

```go
fmt.Println("all children:")
for i := 0; i < fn.ChildCount(); i++ {
	child := fn.Child(i)
	fmt.Printf("  [%d] %s named=%v\n", i, child.Type(lang), child.IsNamed())
}

fmt.Println("named children only:")
for i := 0; i < fn.NamedChildCount(); i++ {
	child := fn.NamedChild(i)
	fmt.Printf("  [%d] %s\n", i, child.Type(lang))
}
```

```text
all children:
  [0] func named=false
  [1] identifier named=true
  [2] parameter_list named=true
  [3] block named=true
named children only:
  [0] identifier
  [1] parameter_list
  [2] block
```

### Named vs. anonymous nodes

`func` shows up in the full child list with `named=false`: it's a keyword, a fixed piece of syntax
the grammar spells out literally. `identifier`, `parameter_list`, and `block` are named nodes —
they correspond to real grammar rules and carry structure of their own. As a rule of thumb, walk
`NamedChild`/`NamedChildCount` when you're analyzing structure and don't want to trip over every
keyword and brace, and fall back to `Child`/`ChildCount` when you need the literal picture — for
example, reconstructing exact source formatting. It's the same distinction `SExpr` uses: that's why
`func` never appears in the S-expression output above.

## Picking a language

`grammars.GoLanguage()` is one of 206 `XLanguage() *gts.Language` functions, one per embedded
grammar: `grammars.PythonLanguage()`, `grammars.RustLanguage()`, `grammars.TypescriptLanguage()`,
and so on for every grammar gotreesitter ships. When you don't know the language ahead of time,
`grammars.AllLanguages()` enumerates every registered grammar as metadata — it doesn't decode any
parse tables, so it's cheap to call even in a hot path:

```go
for _, entry := range grammars.AllLanguages() {
	fmt.Println(entry.Name, entry.Extensions)
}
```

Each `grammars.LangEntry` carries a lazy `Language func() *gts.Language` field alongside its name
and file extensions, so you only pay to decode the grammar you actually use.

In practice, tools rarely hardcode a language function. They resolve one from a filename instead:

```go
entry := grammars.DetectLanguage("main.go")
if entry == nil {
	// no grammar registered for this filename/extension
}
lang := entry.Language()
```

`DetectLanguage` matches exact filenames first (`Dockerfile`, `.bashrc`), then file extensions —
longest suffix first, so `.blade.php` resolves before `.php` — and returns `nil` when nothing
matches.

## Next steps

- [Syntax Trees and Nodes](/docs/syntax-trees-and-nodes) — the rest of the node and tree API:
  fields, cursors, descendant lookup, editing.
- [Queries](/docs/queries) — S-expression pattern matching over a tree.
- [Incremental Parsing](/docs/incremental-parsing) — reparse after an edit without redoing the
  whole file.
