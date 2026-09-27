package docs

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"sync"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"m31labs.dev/gosx"
)

type landingParseSpec struct {
	Name     string
	Label    string
	Filename string
	Source   string
}

type landingParseNode struct {
	ID    string
	Type  string
	Start uint32
	End   uint32
	Depth int
}

type landingParseToken struct {
	Start    uint32
	End      uint32
	NodeID   string
	NodeType string
	Class    string
	Text     string
}

type landingParseExample struct {
	Name     string
	Label    string
	Filename string
	Source   string
	SExpr    string
	Nodes    []landingParseNode
	Tokens   []landingParseToken
}

var landingParseSpecs = []landingParseSpec{
	{
		Name: "go", Label: "Go", Filename: "add.go",
		Source: "func add(left, right int) int {\n\treturn left + right\n}\n",
	},
	{
		Name: "python", Label: "Python", Filename: "add.py",
		Source: "def add(left, right):\n    return left + right\n",
	},
	{
		Name: "rust", Label: "Rust", Filename: "add.rs",
		Source: "fn add(left: i32, right: i32) -> i32 {\n    left + right\n}\n",
	},
	{
		Name: "typescript", Label: "TypeScript", Filename: "add.ts",
		Source: "function add(left: number, right: number): number {\n  return left + right;\n}\n",
	},
	{
		Name: "json", Label: "JSON", Filename: "package.json",
		Source: "{\n  \"parser\": \"gotreesitter\",\n  \"languages\": 206\n}\n",
	},
	{
		Name: "html", Label: "HTML", Filename: "index.html",
		Source: "<article><h1>Tree</h1><p>Go parses source.</p></article>\n",
	},
}

var (
	landingParseCacheOnce sync.Once
	landingParseCache     []landingParseExample
	landingParseCacheErr  error
	landingMarkupOnce     sync.Once
	landingMarkupCache    string
	landingMarkupErr      error
)

func init() {
	landingParseCacheOnce.Do(func() {
		landingParseCache, landingParseCacheErr = buildLandingParseExamples()
	})
	if landingParseCacheErr != nil {
		panic(landingParseCacheErr)
	}
}

func landingParserDemoNode(nonce string) gosx.Node {
	markup, err := landingParserDemoMarkup(nonce)
	if err != nil {
		return gosx.Text("The parser example could not be loaded.")
	}
	return gosx.RawHTML(markup)
}

func landingParserDemoMarkup(nonce string) (string, error) {
	landingParseCacheOnce.Do(func() {
		landingParseCache, landingParseCacheErr = buildLandingParseExamples()
	})
	if landingParseCacheErr != nil {
		return "", landingParseCacheErr
	}
	landingMarkupOnce.Do(func() {
		landingMarkupCache, landingMarkupErr = buildLandingParserDemoMarkup()
	})
	if landingMarkupErr != nil {
		return "", landingMarkupErr
	}
	if nonce == "" {
		return strings.Replace(landingMarkupCache, ` nonce="__GTS_NONCE__"`, "", 1), nil
	}
	return strings.Replace(landingMarkupCache, `__GTS_NONCE__`, html.EscapeString(nonce), 1), nil
}

func buildLandingParserDemoMarkup() (string, error) {
	var b strings.Builder
	b.Grow(16000)
	b.WriteString(`<div class="parser-demo"><style nonce="__GTS_NONCE__">`)
	for _, example := range landingParseCache {
		key := html.EscapeString(example.Name)
		b.WriteString(`.parser-demo:has(#parser-lang-` + key + `:checked) #parser-sample-` + key + `{display:flex;opacity:1;pointer-events:auto;transition:opacity 150ms ease}`)
		for _, node := range example.Nodes {
			selector := `.parser-sample[data-language-panel="` + key + `"]:has(.parser-source-token[data-node-id="` + node.ID + `"]:`
			b.WriteString(selector + `hover) .parser-tree-row[data-node-id="` + node.ID + `"],`)
			b.WriteString(selector + `focus-visible) .parser-tree-row[data-node-id="` + node.ID + `"]`)
			b.WriteString(`{background:var(--parse-active);color:var(--color-ink);box-shadow:inset 4px 0 var(--color-blue)}`)
		}
	}
	b.WriteString(`</style><fieldset class="parser-fieldset"><legend class="sr-only">Choose a language to inspect its parse</legend><div class="parser-tabs">`)
	for i, example := range landingParseCache {
		checked := ""
		if i == 0 {
			checked = " checked"
		}
		id := "parser-lang-" + html.EscapeString(example.Name)
		b.WriteString(`<input class="parser-radio" type="radio" name="parser-language" id="` + id + `" value="` + html.EscapeString(example.Name) + `"` + checked + `><label class="parser-tab" for="` + id + `">` + html.EscapeString(example.Label) + `</label>`)
	}
	b.WriteString(`</div><div class="parser-samples">`)
	for _, example := range landingParseCache {
		key := html.EscapeString(example.Name)
		b.WriteString(`<section id="parser-sample-` + key + `" class="parser-sample" data-language-panel="` + key + `" aria-labelledby="parser-title-` + key + `"><h3 class="parser-sample-title" id="parser-title-` + key + `">` + html.EscapeString(example.Label) + ` parse</h3><div class="parser-sample-grid">`)
		b.WriteString(`<div class="code parser-code-panel"><div class="codehead"><span class="cdot r" aria-hidden="true"></span><span class="cdot y" aria-hidden="true"></span><span class="cdot g" aria-hidden="true"></span><span class="cfile mono">` + html.EscapeString(example.Filename) + `</span><span class="clang">source</span></div><pre class="codebody parser-source mono" aria-label="` + html.EscapeString(example.Label) + ` source code">`)
		b.WriteString(renderLandingParseSource(example))
		b.WriteString(`</pre></div><div class="parser-tree-panel"><div class="panelhd"><span class="ldot c-violet" aria-hidden="true"></span>named-node tree<span class="hlcredit">UTF-8 byte range</span></div><ol class="parser-tree mono" aria-label="` + html.EscapeString(example.Label) + ` named-node tree with byte ranges">`)
		for _, node := range example.Nodes {
			depth := node.Depth
			if depth > 8 {
				depth = 8
			}
			b.WriteString(`<li class="parser-tree-row" data-node-id="` + node.ID + `" data-depth="` + fmt.Sprint(depth) + `"><span class="parser-node-type">` + html.EscapeString(node.Type) + `</span><span class="parser-node-range">[` + fmt.Sprint(node.Start) + `, ` + fmt.Sprint(node.End) + `)</span></li>`)
		}
		b.WriteString(`</ol></div></div></section>`)
	}
	b.WriteString(`</div><p class="parser-demo-credit">Parsed on the docs server with gotreesitter ` + html.EscapeString(PlaygroundGTSVersion()) + `; examples are cached per process. Point to a token or use Tab to follow it to its tree row. Ranges are UTF-8 byte offsets with an exclusive end.</p></fieldset></div>`)
	return b.String(), nil
}

func buildLandingParseExamples() ([]landingParseExample, error) {
	examples := make([]landingParseExample, 0, len(landingParseSpecs))
	for _, spec := range landingParseSpecs {
		entry := grammars.DetectLanguageByName(spec.Name)
		if entry == nil {
			return nil, fmt.Errorf("landing parser example language %q is not registered", spec.Name)
		}
		language := entry.Language()
		if language == nil {
			return nil, fmt.Errorf("landing parser example language %q could not be loaded", spec.Name)
		}
		parser := gts.NewParser(language)
		source := []byte(spec.Source)
		var tree *gts.Tree
		var err error
		if entry.TokenSourceFactory != nil {
			tree, err = parser.ParseWithTokenSourceFactory(source, func(input []byte) (gts.TokenSource, error) {
				return entry.TokenSourceFactory(input, language), nil
			})
		} else {
			tree, err = parser.Parse(source)
		}
		if err != nil {
			return nil, fmt.Errorf("parse %s landing example: %w", spec.Name, err)
		}
		if tree == nil || tree.RootNode() == nil {
			return nil, fmt.Errorf("parse %s landing example returned no root node", spec.Name)
		}
		root := tree.RootNode()
		if root.HasError() {
			tree.Release()
			return nil, fmt.Errorf("parse %s landing example produced an error tree", spec.Name)
		}
		example := landingParseExample{
			Name:     spec.Name,
			Label:    spec.Label,
			Filename: spec.Filename,
			Source:   spec.Source,
			SExpr:    root.SExpr(language),
		}
		nodeIDs := make(map[*gts.Node]string)
		collectLandingParseNodes(root, language, spec.Name, 0, &example.Nodes, nodeIDs)
		collectLandingParseTokens(root, language, source, spec.Name, nodeIDs, "", &example.Tokens)
		tree.Release()
		sort.SliceStable(example.Tokens, func(i, j int) bool {
			if example.Tokens[i].Start == example.Tokens[j].Start {
				return example.Tokens[i].End < example.Tokens[j].End
			}
			return example.Tokens[i].Start < example.Tokens[j].Start
		})
		examples = append(examples, example)
	}
	return examples, nil
}

func collectLandingParseNodes(node *gts.Node, language *gts.Language, languageName string, depth int, nodes *[]landingParseNode, ids map[*gts.Node]string) {
	if node == nil {
		return
	}
	currentDepth := depth
	if node.IsNamed() {
		id := fmt.Sprintf("%s-node-%d", languageName, len(*nodes))
		ids[node] = id
		*nodes = append(*nodes, landingParseNode{
			ID:    id,
			Type:  node.Type(language),
			Start: node.StartByte(),
			End:   node.EndByte(),
			Depth: depth,
		})
		currentDepth++
	}
	for i := 0; i < node.ChildCount(); i++ {
		collectLandingParseNodes(node.Child(i), language, languageName, currentDepth, nodes, ids)
	}
}

func collectLandingParseTokens(node *gts.Node, language *gts.Language, source []byte, languageName string, ids map[*gts.Node]string, parentID string, tokens *[]landingParseToken) {
	if node == nil {
		return
	}
	if id := ids[node]; id != "" {
		parentID = id
	}
	if node.ChildCount() == 0 {
		start, end := node.StartByte(), node.EndByte()
		if parentID != "" && end > start && int(end) <= len(source) {
			class := classify(node, language, source, tkClassifiers[languageName])
			if class == "" {
				class = "tk-pn"
			}
			*tokens = append(*tokens, landingParseToken{
				Start: start, End: end, NodeID: parentID,
				NodeType: node.Type(language), Class: class,
				Text: string(source[start:end]),
			})
		}
		return
	}
	for i := 0; i < node.ChildCount(); i++ {
		collectLandingParseTokens(node.Child(i), language, source, languageName, ids, parentID, tokens)
	}
}

func renderLandingParseSource(example landingParseExample) string {
	var b strings.Builder
	b.Grow(len(example.Source) * 3)
	source := []byte(example.Source)
	var position uint32
	for _, token := range example.Tokens {
		if token.Start < position || token.End > uint32(len(source)) || token.End <= position {
			continue
		}
		if token.Start > position {
			b.WriteString(html.EscapeString(string(source[position:token.Start])))
		}
		label := token.Text + ", " + token.NodeType + ", bytes " + fmt.Sprint(token.Start) + " to " + fmt.Sprint(token.End)
		b.WriteString(`<span class="parser-source-token ` + html.EscapeString(token.Class) + `" data-node-id="` + html.EscapeString(token.NodeID) + `" tabindex="0" aria-label="` + html.EscapeString(label) + `">`)
		b.WriteString(html.EscapeString(string(source[token.Start:token.End])))
		b.WriteString(`</span>`)
		position = token.End
	}
	if position < uint32(len(source)) {
		b.WriteString(html.EscapeString(string(source[position:])))
	}
	return b.String()
}
