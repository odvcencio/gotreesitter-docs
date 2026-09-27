---
title: Getting Started
description: Try gotreesitter in the browser, then parse and inspect your first file in Go.
nav_group: Introduction
order: 2
layout: steps
---

Start in the [browser playground](/playground): choose Go, edit the sample, and inspect its
syntax tree. You can try a grammar before installing anything.

## Requirements

You need Go 1.22.0 or newer for the v0.55.1 module. You can confirm the minimum in its
[go.mod](https://github.com/odvcencio/gotreesitter/blob/v0.55.1/go.mod).

## Install

Add v0.55.1 to your module:

```sh
go get github.com/odvcencio/gotreesitter@v0.55.1
```

You get the parser and all 206 built-in grammars in the same module.

## Parse a Go file

This complete program parses a small Go file and prints its named syntax tree:

```go title=main.go
package main

import (
	"fmt"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func main() {
	src := []byte("package main\n\nfunc main() {}\n")
	lang := grammars.GoLanguage()
	parser := gts.NewParser(lang)

	tree, err := parser.Parse(src)
	if err != nil {
		panic(err)
	}
	defer tree.Release()

	root := tree.RootNode()
	fmt.Println(root.SExpr(lang))
}
```

```text
(source_file (package_clause (package_identifier)) (function_declaration (identifier) (parameter_list) (block)))
```

You load Go's parse tables with `grammars.GoLanguage` and bind a parser to them with
`NewParser`. `Parse` returns a tree, and `RootNode` gives you its top node. `SExpr` prints named
nodes while leaving punctuation and keywords out; call `Release` when you are done with the tree.

## Read a node

Use `NamedChild` to select the function declaration. This program prints its node type, byte
range, and source text:

```go title=read-node.go
package main

import (
	"fmt"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func main() {
	src := []byte("package main\n\nfunc main() {}\n")
	lang := grammars.GoLanguage()
	tree, err := gts.NewParser(lang).Parse(src)
	if err != nil {
		panic(err)
	}
	defer tree.Release()

	fn := tree.RootNode().NamedChild(1)
	fmt.Println(fn.Type(lang))
	fmt.Println(fn.StartByte(), fn.EndByte())
	fmt.Println(fn.Text(src))
}
```

```text
function_declaration
14 28
func main() {}
```

You see the declaration name, its byte offsets into `src`, and the source covered by that node.
`Type` uses the language to turn a numeric symbol into a name. `NamedChild` skips punctuation;
`Child` includes every grammar child.

## Walk children

Use `Child` when you want to see keywords and punctuation as well as named nodes:

```go title=walk-children.go
package main

import (
	"fmt"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func main() {
	src := []byte("package main\n\nfunc main() {}\n")
	lang := grammars.GoLanguage()
	tree, err := gts.NewParser(lang).Parse(src)
	if err != nil {
		panic(err)
	}
	defer tree.Release()

	fn := tree.RootNode().NamedChild(1)
	for i := 0; i < fn.ChildCount(); i++ {
		child := fn.Child(i)
		fmt.Printf("%s named=%v\n", child.Type(lang), child.IsNamed())
	}
}
```

```text
func named=false
identifier named=true
parameter_list named=true
block named=true
```

You see the `func` keyword, followed by three named nodes. Use `NamedChildCount` and `NamedChild`
when you only need the grammar structure.

## Choose a language by filename

When you do not know the language ahead of time, ask the registry to detect it:

```go
entry := grammars.DetectLanguage("main.go")
if entry == nil {
	return
}
lang := entry.Language()
parser := gts.NewParser(lang)
tree, err := parser.Parse(src)
if err != nil {
	panic(err)
}
defer tree.Release()
```

`DetectLanguage` checks registered filenames and extensions. It returns nil when no grammar
matches; `entry.Language()` loads the matching grammar.

## Check for syntax errors

A file with syntax errors can still produce a tree. Check its root node:

```go
tree, err := parser.Parse([]byte("package main\nfunc main( {\n"))
if err != nil {
	panic(err)
}
defer tree.Release()
if tree.RootNode().HasError() {
	fmt.Println("the tree contains a syntax error")
}
```

Errors returned by `Parse` report setup problems. `HasError` reports `ERROR` or `MISSING` nodes in
the parsed tree.

## Keep going

- [Queries](/docs/queries) find nodes by structure.
- [Incremental Parsing](/docs/incremental-parsing) reuses a previous tree after an edit.
- [Syntax Highlighting](/docs/syntax-highlighting) turns query captures into source ranges.
- [The road to v1](/docs/v1) explains the engine transition and release gates.
