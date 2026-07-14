package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const (
	maxTreeNodes    = 20_000
	maxTreeRows     = 4_000
	maxQueryMatches = 500
)

type runtimeLanguage struct {
	name               string
	language           *gotreesitter.Language
	tokenSourceFactory func([]byte) gotreesitter.TokenSource
	highlighter        *gotreesitter.Highlighter
	highlightErr       error
}

func loadRuntimeLanguage(name string, blob []byte, highlightQuery string) (*runtimeLanguage, error) {
	language, err := grammars.LoadLanguage(name, blob)
	if err != nil {
		return nil, fmt.Errorf("load %s grammar: %w", name, err)
	}
	loaded := &runtimeLanguage{name: name, language: language}
	if entry := grammars.DetectLanguageByName(name); entry != nil && entry.TokenSourceFactory != nil {
		loaded.tokenSourceFactory = func(source []byte) gotreesitter.TokenSource {
			return entry.TokenSourceFactory(source, language)
		}
	}
	if strings.TrimSpace(highlightQuery) != "" {
		options := []gotreesitter.HighlighterOption{}
		if loaded.tokenSourceFactory != nil {
			options = append(options, gotreesitter.WithTokenSourceFactory(loaded.tokenSourceFactory))
		}
		loaded.highlighter, loaded.highlightErr = gotreesitter.NewHighlighter(language, highlightQuery, options...)
	}
	return loaded, nil
}

func (l *runtimeLanguage) parseUTF16(parser *gotreesitter.Parser, source []uint16, oldTree *gotreesitter.Tree) (*gotreesitter.Tree, error) {
	if parser == nil {
		parser = gotreesitter.NewParser(l.language)
	}
	if l.tokenSourceFactory == nil {
		if oldTree != nil {
			return parser.ParseIncrementalUTF16(source, oldTree)
		}
		return parser.ParseUTF16(source)
	}
	factory := func(source []byte) (gotreesitter.TokenSource, error) {
		return l.tokenSourceFactory(source), nil
	}
	if oldTree != nil {
		return parser.ParseIncrementalUTF16WithTokenSourceFactory(source, oldTree, factory)
	}
	return parser.ParseUTF16WithTokenSourceFactory(source, factory)
}

type runtimeDocument struct {
	language *runtimeLanguage
	parser   *gotreesitter.Parser
	tree     *gotreesitter.Tree
	source   string
	units    []uint16
	revision uint64
}

func (d *runtimeDocument) open(language *runtimeLanguage, source string) error {
	d.close()
	units := toUTF16(source)
	parser := gotreesitter.NewParser(language.language)
	tree, err := language.parseUTF16(parser, units, nil)
	if err != nil {
		if tree != nil {
			tree.Release()
		}
		return err
	}
	if tree == nil {
		return fmt.Errorf("parse returned no tree")
	}
	d.language = language
	d.parser = parser
	d.tree = tree
	d.source = source
	d.units = units
	d.revision = 1
	return nil
}

// update performs one incremental edit against the retained parser/tree. If
// the incremental parse fails after mutating the old tree, it falls back to a
// fresh parse and replaces the complete document transactionally.
func (d *runtimeDocument) update(source string) error {
	if d.tree == nil || d.language == nil {
		return fmt.Errorf("document is not open")
	}
	units := toUTF16(source)
	edit, changed := utf16EditBetween(d.units, units)
	if !changed {
		return nil
	}
	if !d.tree.EditUTF16(edit, units) {
		return fmt.Errorf("edit does not align to a UTF-16 boundary")
	}
	oldTree := d.tree
	newTree, err := d.language.parseUTF16(d.parser, units, oldTree)
	if err != nil || newTree == nil {
		if newTree != nil && newTree != oldTree {
			newTree.Release()
		}
		freshParser := gotreesitter.NewParser(d.language.language)
		newTree, err = d.language.parseUTF16(freshParser, units, nil)
		if err != nil || newTree == nil {
			if newTree != nil {
				newTree.Release()
			}
			oldTree.Release()
			d.tree = nil
			if err != nil {
				return fmt.Errorf("incremental update failed and document closed: %w", err)
			}
			return fmt.Errorf("incremental update returned no tree and document closed")
		}
		d.parser = freshParser
	}
	d.tree = newTree
	d.source = source
	d.units = units
	d.revision++
	if oldTree != newTree {
		oldTree.Release()
	}
	return nil
}

func (d *runtimeDocument) close() {
	if d.tree != nil {
		d.tree.Release()
	}
	*d = runtimeDocument{}
}

func toUTF16(source string) []uint16 {
	return utf16.Encode([]rune(source))
}

func utf16EditBetween(oldSource, newSource []uint16) (gotreesitter.UTF16Edit, bool) {
	prefix := 0
	for prefix < len(oldSource) && prefix < len(newSource) && oldSource[prefix] == newSource[prefix] {
		prefix++
	}
	if prefix == len(oldSource) && prefix == len(newSource) {
		return gotreesitter.UTF16Edit{
			StartCodeUnit:  uint32(prefix),
			OldEndCodeUnit: uint32(prefix),
			NewEndCodeUnit: uint32(prefix),
		}, false
	}
	for prefix > 0 && (!utf16Boundary(oldSource, prefix) || !utf16Boundary(newSource, prefix)) {
		prefix--
	}

	oldEnd, newEnd := len(oldSource), len(newSource)
	for oldEnd > prefix && newEnd > prefix && oldSource[oldEnd-1] == newSource[newEnd-1] {
		oldEnd--
		newEnd--
	}
	for oldEnd < len(oldSource) && newEnd < len(newSource) &&
		(!utf16Boundary(oldSource, oldEnd) || !utf16Boundary(newSource, newEnd)) {
		oldEnd++
		newEnd++
	}

	return gotreesitter.UTF16Edit{
		StartCodeUnit:  uint32(prefix),
		OldEndCodeUnit: uint32(oldEnd),
		NewEndCodeUnit: uint32(newEnd),
	}, true
}

func utf16Boundary(source []uint16, offset int) bool {
	if offset <= 0 || offset >= len(source) {
		return true
	}
	return !isLeadSurrogate(source[offset-1]) || !isTrailSurrogate(source[offset])
}

func isLeadSurrogate(unit uint16) bool  { return unit >= 0xd800 && unit <= 0xdbff }
func isTrailSurrogate(unit uint16) bool { return unit >= 0xdc00 && unit <= 0xdfff }

type viewNode struct {
	typ       string
	field     string
	start     uint32
	end       uint32
	start16   uint32
	end16     uint32
	named     bool
	missing   bool
	isError   bool
	truncated bool
	children  []*viewNode
}

func buildViewTree(tree *gotreesitter.Tree, language *gotreesitter.Language, limit int) *viewNode {
	remaining := limit
	omitted := false
	var build func(*gotreesitter.Node, string) *viewNode
	build = func(node *gotreesitter.Node, field string) *viewNode {
		if node == nil {
			return nil
		}
		if remaining <= 0 {
			omitted = true
			return nil
		}
		remaining--
		start16, end16 := node.StartByte(), node.EndByte()
		if sourceRange, ok := tree.UTF16RangeForNode(node); ok {
			start16, end16 = sourceRange.StartCodeUnit, sourceRange.EndCodeUnit
		}
		out := &viewNode{
			typ:     node.Type(language),
			field:   field,
			start:   node.StartByte(),
			end:     node.EndByte(),
			start16: start16,
			end16:   end16,
			named:   node.IsNamed(),
			missing: node.IsMissing(),
			isError: node.IsError(),
		}
		for index := 0; index < node.ChildCount(); index++ {
			if remaining <= 0 {
				omitted = true
				break
			}
			if child := build(node.Child(index), node.FieldNameForChild(index, language)); child != nil {
				out.children = append(out.children, child)
			}
		}
		return out
	}
	if tree == nil {
		return nil
	}
	root := build(tree.RootNode(), "")
	if root != nil {
		root.truncated = omitted
	}
	return root
}

type tokenRange struct {
	start   uint32
	end     uint32
	capture string
}

func documentTokenRanges(document *runtimeDocument) []tokenRange {
	if document == nil || document.tree == nil || document.language == nil || document.language.highlighter == nil {
		return nil
	}
	highlights := document.language.highlighter.HighlightTreeUTF16(document.tree)
	ranges := make([]tokenRange, 0, len(highlights))
	for _, highlight := range highlights {
		start, startOK := document.tree.UTF8ByteForUTF16Offset(highlight.StartCodeUnit)
		end, endOK := document.tree.UTF8ByteForUTF16Offset(highlight.EndCodeUnit)
		if !startOK || !endOK || end <= start {
			continue
		}
		ranges = append(ranges, tokenRange{start: start, end: end, capture: highlight.Capture})
	}
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].start < ranges[j].start || ranges[i].start == ranges[j].start && ranges[i].end < ranges[j].end
	})
	return ranges
}

type queryCapture struct {
	name    string
	typ     string
	start   uint32
	end     uint32
	start16 uint32
	end16   uint32
}

type queryResult struct {
	captures  []queryCapture
	truncated bool
	Err       error
	ErrOffset int
}

var queryErrorOffsetPattern = regexp.MustCompile(`at position (\d+)`)

func queryDocument(document *runtimeDocument, source string) queryResult {
	if document == nil || document.tree == nil || document.language == nil {
		return queryResult{Err: fmt.Errorf("document is not open"), ErrOffset: -1}
	}
	query, err := gotreesitter.NewQuery(source, document.language.language)
	if err != nil {
		result := queryResult{Err: err, ErrOffset: -1}
		if match := queryErrorOffsetPattern.FindStringSubmatch(err.Error()); len(match) == 2 {
			if offset, convErr := strconv.Atoi(match[1]); convErr == nil {
				result.ErrOffset = offset
			}
		}
		return result
	}
	cursor := query.Exec(document.tree.RootNode(), document.language.language, document.tree.Source())
	cursor.SetMatchLimit(maxQueryMatches)
	result := queryResult{ErrOffset: -1}
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			if capture.Node == nil {
				continue
			}
			start16, end16 := capture.Node.StartByte(), capture.Node.EndByte()
			if sourceRange, ok := document.tree.UTF16RangeForNode(capture.Node); ok {
				start16, end16 = sourceRange.StartCodeUnit, sourceRange.EndCodeUnit
			}
			result.captures = append(result.captures, queryCapture{
				name:    capture.Name,
				typ:     capture.Node.Type(document.language.language),
				start:   capture.Node.StartByte(),
				end:     capture.Node.EndByte(),
				start16: start16,
				end16:   end16,
			})
		}
	}
	result.truncated = cursor.DidExceedMatchLimit()
	return result
}

var (
	shebangPython     = regexp.MustCompile(`python`)
	shebangNode       = regexp.MustCompile(`node`)
	shebangRuby       = regexp.MustCompile(`ruby`)
	shebangShell      = regexp.MustCompile(`(ba|z)?sh`)
	phpSignature      = regexp.MustCompile(`^\s*<\?php`)
	htmlSignature     = regexp.MustCompile(`(?i)^\s*(<!DOCTYPE|<html)`)
	jsonSignature     = regexp.MustCompile(`^\s*[\{\[]`)
	jsonProperty      = regexp.MustCompile(`"[^"\n]*"\s*:`)
	goPackage         = regexp.MustCompile(`\bpackage\s+\w+`)
	goFunc            = regexp.MustCompile(`\bfunc\b`)
	cInclude          = regexp.MustCompile(`#[[:space:]]*include\s*[<"]`)
	rustFunc          = regexp.MustCompile(`\bfn\s+\w+\s*\(`)
	rustMarker        = regexp.MustCompile(`\blet\s+mut\b|::|->`)
	pythonFunc        = regexp.MustCompile(`(?m)^\s*def\s+\w+\s*\(.*\)\s*:`)
	sqlVerb           = regexp.MustCompile(`(?im)^\s*(SELECT|INSERT|UPDATE|DELETE|CREATE)\s`)
	sqlClause         = regexp.MustCompile(`(?i)\b(FROM|INTO|TABLE|SET|WHERE)\b`)
	markdownSignature = regexp.MustCompile("(?m)^```|^#{1,6}\\s+\\S")
)

func heuristicDetect(source string) string {
	head := source
	if len(head) > 4096 {
		head = head[:4096]
		for !utf8.ValidString(head) {
			head = head[:len(head)-1]
		}
	}
	firstLine := head
	if newline := strings.IndexByte(head, '\n'); newline >= 0 {
		firstLine = head[:newline+1]
	}
	if strings.HasPrefix(firstLine, "#!") {
		switch {
		case shebangPython.MatchString(firstLine):
			return "python"
		case shebangNode.MatchString(firstLine):
			return "javascript"
		case shebangRuby.MatchString(firstLine):
			return "ruby"
		case shebangShell.MatchString(firstLine):
			return "bash"
		}
	}
	switch {
	case phpSignature.MatchString(head):
		return "php"
	case htmlSignature.MatchString(head):
		return "html"
	case jsonSignature.MatchString(head) && jsonProperty.MatchString(head):
		return "json"
	case goPackage.MatchString(head) && goFunc.MatchString(head):
		return "go"
	case cInclude.MatchString(head):
		return "c"
	case rustFunc.MatchString(head) && rustMarker.MatchString(head):
		return "rust"
	case pythonFunc.MatchString(head):
		return "python"
	case sqlVerb.MatchString(head) && sqlClause.MatchString(head):
		return "sql"
	case markdownSignature.MatchString(head):
		return "markdown"
	default:
		return ""
	}
}

var tokenClasses = map[string]string{
	"keyword": "tk-kw", "function": "tk-fn", "method": "tk-fn", "constructor": "tk-fn",
	"string": "tk-str", "char": "tk-str", "escape": "tk-str",
	"number": "tk-num", "float": "tk-num", "integer": "tk-num",
	"boolean": "tk-kw", "constant": "tk-kw", "type": "tk-ty", "tag": "tk-ty",
	"attribute": "tk-ty", "comment": "tk-cm", "operator": "tk-op",
	"punctuation": "tk-pn", "delimiter": "tk-pn", "variable": "tk-id",
	"property": "tk-id", "parameter": "tk-id", "field": "tk-id", "label": "tk-id",
}

func tokenClass(capture string) string {
	root, _, _ := strings.Cut(capture, ".")
	return tokenClasses[root]
}

func captureColor(name string) int {
	var hash uint32 = 5381
	for _, r := range name {
		hash = (hash*33 ^ uint32(r))
	}
	return int(hash % 8)
}
