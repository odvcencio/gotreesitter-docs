---
title: Getting Started
description: Try gotreesitter in the browser, then parse and inspect your first file in Go.
nav_group: Introduction
order: 2
layout: steps
---

Try a grammar before you install anything in the [browser playground](/playground). Choose Go,
edit the sample, and inspect the syntax tree.

## Requirements

For the v0.55.1 module, use Go 1.22.0 or newer. That is the go directive in its
[go.mod](https://github.com/odvcencio/gotreesitter/blob/v0.55.1/go.mod).

## Install

Add gotreesitter v0.55.1 to your module:

```sh
go get github.com/odvcencio/gotreesitter@v0.55.1
```

The parser and the 206 built-in grammars are in the same module.

## Parse a Go file

This program parses a small Go file and prints its named syntax tree:

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

Running it prints:

```text
(source_file (package_clause (package_identifier)) (function_declaration (identifier) (parameter_list) (block)))
```

grammars.GoLanguage loads Go's parse tables. NewParser binds a parser to that language.
Parse returns a tree, and RootNode gives you the top node. SExpr prints named nodes;
punctuation and keywords are left out. Release a tree when you are done with it.

## Read a node

Use a named child to find the function declaration. Node positions are byte offsets into src,
and Text returns the source covered by that node:

```go
fn := root.NamedChild(1)
fmt.Println(fn.Type(lang))
fmt.Println(fn.StartByte(), fn.EndByte())
fmt.Println(fn.Text(src))
```

Type uses the language to turn a numeric symbol into a name. NamedChild skips punctuation;
Child includes every grammar child.

## Walk children

Walk every child when you need to see keywords and punctuation too:

```go
for i := 0; i < fn.ChildCount(); i++ {
	child := fn.Child(i)
	fmt.Printf("%s named=%v\n", child.Type(lang), child.IsNamed())
}
```

Use NamedChildCount and NamedChild when you only need grammar structure.

## Choose a language by filename

When a file's language is not known ahead of time, use the registry:

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

DetectLanguage checks registered filenames and extensions. It returns nil when no grammar
matches. Calling entry.Language loads that grammar.

## Check for syntax errors

A source file with syntax errors can still produce a tree. Check the root node:

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

Parse errors report setup problems. HasError reports ERROR or MISSING nodes in the parsed
tree.

## Keep going

- [Queries](/docs/queries) find nodes by structure.
- [Incremental Parsing](/docs/incremental-parsing) reuses a previous tree after an edit.
- [Syntax Highlighting](/docs/syntax-highlighting) turns query captures into source ranges.
- [The road to v1](/docs/v1) explains the engine transition and release gates.
