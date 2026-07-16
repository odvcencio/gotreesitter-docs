package playgroundengine

import (
	"fmt"
	"strconv"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const (
	MaxSourceBytes = 64 << 10
	MaxTreeRows    = 1200
	MaxCaptures    = 500
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

type Capture struct {
	Name  string
	Text  string
	Match int
	Range string
}

type Result struct {
	TreeRows   []TreeRow
	Captures   []Capture
	NodeCount  int
	HasErrors  bool
	ParseError string
	QueryError string
}

// Parse executes gotreesitter entirely inside its caller. In production the
// only caller is the browser WebAssembly engine; the docs server does not
// import this package or receive editor contents.
func Parse(source, querySource, languageName string, includeAnonymous bool) (result Result) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result.ParseError = fmt.Sprintf("parser failed: %v", recovered)
		}
	}()

	entry := grammars.DetectLanguageByName(strings.TrimSpace(languageName))
	if entry == nil {
		result.ParseError = "unknown language: " + languageName
		return result
	}
	lang := entry.Language()
	if lang == nil {
		result.ParseError = "language could not be loaded: " + languageName
		return result
	}
	return ParseLanguage(source, querySource, entry.Name, lang, includeAnonymous)
}

// ParseLanguage parses with an already-loaded grammar. The browser engine uses
// this entry point after fetching and caching a content-hashed grammar blob.
func ParseLanguage(source, querySource, languageName string, lang *gts.Language, includeAnonymous bool) (result Result) {
	result.TreeRows = []TreeRow{}
	result.Captures = []Capture{}
	defer func() {
		if recovered := recover(); recovered != nil {
			result.ParseError = fmt.Sprintf("parser failed: %v", recovered)
		}
	}()
	if len(source) > MaxSourceBytes {
		result.ParseError = "source must be 64 KiB or smaller"
		return result
	}
	if lang == nil {
		result.ParseError = "language is not loaded"
		return result
	}
	parser := gts.NewParser(lang)
	var tree *gts.Tree
	var err error
	entry := grammars.DetectLanguageByName(languageName)
	if entry != nil && entry.TokenSourceFactory != nil {
		tree, err = parser.ParseWithTokenSourceFactory([]byte(source), func(src []byte) (gts.TokenSource, error) {
			return entry.TokenSourceFactory(src, lang), nil
		})
	} else {
		tree, err = parser.Parse([]byte(source))
	}
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
	if strings.TrimSpace(querySource) == "" {
		return result
	}
	query, err := gts.NewQuery(querySource, lang)
	if err != nil {
		result.QueryError = err.Error()
		return result
	}
	for matchIndex, match := range query.Execute(tree) {
		for _, capture := range match.Captures {
			if capture.Node == nil {
				continue
			}
			result.Captures = append(result.Captures, Capture{
				Name:  capture.Name,
				Text:  capture.Text([]byte(source)),
				Match: matchIndex + 1,
				Range: formatRange(capture.Node.StartPoint(), capture.Node.EndPoint()),
			})
			if len(result.Captures) >= MaxCaptures {
				return result
			}
		}
	}
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
		className := "pg-tline"
		if !node.IsNamed() {
			className += " pg-anonrow"
		}
		if node.IsError() || node.IsMissing() {
			className += " pg-err"
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
