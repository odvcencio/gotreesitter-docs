// Package authoringengine implements the grammar-authoring playground's
// author -> compile -> parse loop: import a tree-sitter grammar.json,
// compile it in-memory with grammargen, and parse a sample source against
// the freshly generated language. In production the only caller is the
// browser WebAssembly engine (cmd/authoring-wasm); the docs server does not
// import this package or receive editor contents.
package authoringengine

import (
	"fmt"
	"strconv"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

const (
	// MaxGrammarBytes caps the grammar.json textarea. Phase 0 targets small,
	// sub-millisecond-to-generate grammars (calc/json-shaped); a real Web
	// Worker + generation-time budget is Phase 1.
	MaxGrammarBytes = 64 << 10
	MaxSourceBytes  = 64 << 10
	MaxTreeRows     = 1200
)

type TreeRow struct {
	Class   string
	Depth   string
	Level   int
	Field   string
	Type    string
	Range   string
	Missing bool
}

// Result carries the outcome of one compile+parse pass. At most one of
// ImportError/GenerateError/ParseError is set for a given failing run; all
// three are empty on success.
type Result struct {
	TreeRows      []TreeRow
	NodeCount     int
	HasErrors     bool
	GrammarName   string
	ImportError   string
	GenerateError string
	ParseError    string
}

// Compile runs the full grammar.json -> Grammar -> Language -> Tree loop
// entirely inside its caller. It never touches the filesystem or network:
// grammargen.ImportGrammarJSON and grammargen.GenerateLanguage operate on
// in-memory bytes/structs only, so this is safe to call from the browser
// WebAssembly engine on every keystroke (subject to the caller's own
// debounce).
func Compile(grammarJSON, source string, includeAnonymous bool) (result Result) {
	result.TreeRows = []TreeRow{}
	defer func() {
		if recovered := recover(); recovered != nil {
			result.GenerateError = fmt.Sprintf("grammar compiler panicked: %v", recovered)
		}
	}()

	if len(grammarJSON) > MaxGrammarBytes {
		result.ImportError = "grammar.json must be 64 KiB or smaller"
		return result
	}
	if len(source) > MaxSourceBytes {
		result.ParseError = "sample source must be 64 KiB or smaller"
		return result
	}

	g, err := grammargen.ImportGrammarJSON([]byte(grammarJSON))
	if err != nil {
		result.ImportError = err.Error()
		return result
	}
	result.GrammarName = g.Name

	lang, err := grammargen.GenerateLanguage(g)
	if err != nil {
		result.GenerateError = err.Error()
		return result
	}
	if lang == nil {
		result.GenerateError = "grammar compiled to a nil language"
		return result
	}

	parser := gts.NewParser(lang)
	tree, err := parser.Parse([]byte(source))
	if err != nil {
		result.ParseError = err.Error()
		return result
	}
	if tree == nil || tree.RootNode() == nil {
		result.ParseError = "parser returned no syntax tree"
		return result
	}
	defer tree.Release()

	appendTreeRows(tree.RootNode(), lang, includeAnonymous, 0, "", &result)
	return result
}

func appendTreeRows(node *gts.Node, lang *gts.Language, includeAnonymous bool, depth int, field string, result *Result) {
	if node == nil || len(result.TreeRows) >= MaxTreeRows {
		return
	}
	result.NodeCount++
	if node.IsError() || node.IsMissing() {
		result.HasErrors = true
	}
	if includeAnonymous || node.IsNamed() {
		className := "ag-tline"
		if !node.IsNamed() {
			className += " ag-anonrow"
		}
		if node.IsError() || node.IsMissing() {
			className += " ag-err-row"
		}
		result.TreeRows = append(result.TreeRows, TreeRow{
			Class:   className,
			Depth:   strings.Repeat("  ", depth),
			Level:   depth + 1,
			Field:   field,
			Type:    node.Type(lang),
			Range:   formatRange(node.StartPoint(), node.EndPoint()),
			Missing: node.IsMissing(),
		})
	}
	for i := 0; i < node.ChildCount(); i++ {
		appendTreeRows(node.Child(i), lang, includeAnonymous, depth+1, node.FieldNameForChild(i, lang), result)
		if len(result.TreeRows) >= MaxTreeRows {
			return
		}
	}
}

func formatRange(start, end gts.Point) string {
	return strconv.FormatUint(uint64(start.Row+1), 10) + ":" + strconv.FormatUint(uint64(start.Column+1), 10) + "–" +
		strconv.FormatUint(uint64(end.Row+1), 10) + ":" + strconv.FormatUint(uint64(end.Column+1), 10)
}
