package docs

// Phase B syntax highlighter for markdown fenced code blocks. Rather than
// bringing in an external highlighter (chroma etc.), this reuses the site's
// own subject matter: gotreesitter itself parses each fenced snippet and a
// small per-language classifier table (adapted from the same idea mdpp's
// built-in highlighter uses internally, see github.com/odvcencio/mdpp
// highlight_native.go/highlight_classify.go) maps grammar node types to the
// site's `tk-*` token classes instead of mdpp's own `hl-*` ones. public/docs.css
// scopes token colors as `.code .tk-kw`, so the
// classes below only need to be bare `tk-*` — renderCodeBlock (render_blocks.go)
// is responsible for making sure the highlighted body always lands inside a
// `.code` container.
//
// Covers go, javascript, typescript/tsx, python, json, bash — the "at least"
// list from the Phase B task — plus every other gotreesitter grammar via a
// generic structural fallback (classifyGeneric) when no per-language table
// exists. Unrecognized fence languages (or a fence with no gotreesitter
// grammar match) fall back to a plain, unhighlighted `.codebody` — never a
// render error.
import (
	"html"
	"sort"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// tkClassifier is a per-language mapping from tree-sitter node types (and
// selected keyword text) to a `tk-*` highlight class. callParent names the
// node type whose first child is a call's callee, used by classifyCall to
// paint function/method names with tk-fn.
type tkClassifier struct {
	exact      map[string]string
	keywords   map[string]bool
	callParent string
}

func kwSet(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

var goTk = &tkClassifier{
	exact: map[string]string{
		"comment":                    "tk-cm",
		"interpreted_string_literal": "tk-str",
		"raw_string_literal":         "tk-str",
		"rune_literal":               "tk-str",
		"escape_sequence":            "tk-str",
		"string_content":             "tk-str",
		"int_literal":                "tk-num",
		"float_literal":              "tk-num",
		"imaginary_literal":          "tk-num",
		"true":                       "tk-kw",
		"false":                      "tk-kw",
		"nil":                        "tk-kw",
		"iota":                       "tk-kw",
		"type_identifier":            "tk-ty",
		"package_identifier":         "tk-ty",
	},
	keywords: kwSet(
		"break", "case", "chan", "const", "continue", "default", "defer",
		"else", "fallthrough", "for", "func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return", "select", "struct",
		"switch", "type", "var",
	),
	callParent: "call_expression",
}

var pythonTk = &tkClassifier{
	exact: map[string]string{
		"comment":         "tk-cm",
		"string":          "tk-str",
		"string_start":    "tk-str",
		"string_content":  "tk-str",
		"string_end":      "tk-str",
		"escape_sequence": "tk-str",
		"integer":         "tk-num",
		"float":           "tk-num",
		"true":            "tk-kw",
		"false":           "tk-kw",
		"none":            "tk-kw",
		"type":            "tk-ty",
	},
	keywords: kwSet(
		"def", "class", "return", "if", "elif", "else", "for", "while",
		"break", "continue", "pass", "import", "from", "as", "with", "try",
		"except", "finally", "raise", "lambda", "global", "nonlocal", "yield",
		"async", "await", "and", "or", "not", "in", "is",
		"True", "False", "None",
	),
	callParent: "call",
}

var jsTk = &tkClassifier{
	exact: map[string]string{
		"comment":         "tk-cm",
		"string":          "tk-str",
		"template_string": "tk-str",
		"string_fragment": "tk-str",
		"regex":           "tk-str",
		"escape_sequence": "tk-str",
		"number":          "tk-num",
		"true":            "tk-kw",
		"false":           "tk-kw",
		"null":            "tk-kw",
		"undefined":       "tk-kw",
	},
	keywords: kwSet(
		"function", "const", "let", "var", "class", "extends", "import",
		"export", "from", "default", "if", "else", "for", "while", "do",
		"return", "break", "continue", "switch", "case", "try", "catch",
		"finally", "throw", "new", "this", "super", "typeof", "instanceof",
		"in", "of", "async", "await", "yield", "void", "delete",
	),
	callParent: "call_expression",
}

var tsTk = &tkClassifier{
	exact: map[string]string{
		"comment":         "tk-cm",
		"string":          "tk-str",
		"template_string": "tk-str",
		"string_fragment": "tk-str",
		"regex":           "tk-str",
		"escape_sequence": "tk-str",
		"number":          "tk-num",
		"true":            "tk-kw",
		"false":           "tk-kw",
		"null":            "tk-kw",
		"undefined":       "tk-kw",
		"type_identifier": "tk-ty",
		"predefined_type": "tk-ty",
	},
	keywords: kwSet(
		"function", "const", "let", "var", "class", "extends", "import",
		"export", "from", "default", "if", "else", "for", "while", "do",
		"return", "break", "continue", "switch", "case", "try", "catch",
		"finally", "throw", "new", "this", "super", "typeof", "instanceof",
		"in", "of", "async", "await", "yield", "void", "delete",
		"type", "interface", "enum", "namespace", "declare", "readonly",
		"private", "public", "protected", "abstract", "implements", "as",
		"satisfies", "keyof", "is",
	),
	callParent: "call_expression",
}

var bashTk = &tkClassifier{
	exact: map[string]string{
		"comment":              "tk-cm",
		"string":               "tk-str",
		"raw_string":           "tk-str",
		"ansi_c_string":        "tk-str",
		"expansion":            "tk-str",
		"simple_expansion":     "tk-str",
		"command_substitution": "tk-str",
		"number":               "tk-num",
		// command_name colors the invoked program itself (go, bash, cd, ...).
		// Without this, a bare command with no string/number/comment token —
		// e.g. "go get github.com/odvcencio/gotreesitter" or "bash
		// script.sh arg" (getting-started.md, contributing.md) — classifies
		// zero spans under the table above and silently falls back to plain
		// text even though it parsed as real bash. This is confirmed real
		// tree-sitter-bash structure (bash's grammar always wraps a
		// command's first word in a command_name node), not a heuristic
		// guess.
		"command_name": "tk-fn",
	},
	keywords: kwSet(
		"if", "then", "else", "elif", "fi", "for", "in", "do", "done",
		"while", "until", "case", "esac", "function", "return", "break",
		"continue", "local", "export", "readonly", "declare", "unset",
		"source",
	),
}

var jsonTk = &tkClassifier{
	exact: map[string]string{
		"comment":         "tk-cm",
		"string":          "tk-str",
		"string_content":  "tk-str",
		"escape_sequence": "tk-str",
		"number":          "tk-num",
		"true":            "tk-kw",
		"false":           "tk-kw",
		"null":            "tk-kw",
	},
}

// schemeTk backs content/docs/*.md's ```scheme fences, which are tree-sitter
// *query* examples (queries.md, code-navigation.md, language-injection.md's
// `.scm` tags/highlights/injections samples) rather than general-purpose
// Scheme programs — gotreesitter has no dedicated tree-sitter-query grammar,
// so DetectLanguageByName("scheme") is the closest real grammar available,
// and it parses query syntax cleanly (confirmed directly: `(node field:
// (child) @capture)` round-trips as nested Scheme `list`/`symbol` nodes with
// no ERROR besides the `#pred?` predicate token, which Scheme's own `#`
// reader syntax doesn't cover — see classifySchemeSymbol below).
//
// classify() skips classifyGeneric entirely once a language has *any*
// registered tkClassifier (see classify's `cls == nil` branch), so this
// table must carry its own literal/comment/string mappings rather than
// relying on classifyGeneric's substring-on-"string" fallback the way an
// unregistered language would.
var schemeTk = &tkClassifier{
	exact: map[string]string{
		"comment":   "tk-cm",
		"string":    "tk-str",
		"character": "tk-str",
		"number":    "tk-num",
		"boolean":   "tk-kw",
	},
}

// tkClassifiers maps a canonical gotreesitter grammar name (grammars.LangEntry.Name)
// to its classifier table. Any language absent here still gets real,
// tree-sitter-driven highlighting through classifyGeneric.
var tkClassifiers = map[string]*tkClassifier{
	"go":         goTk,
	"python":     pythonTk,
	"javascript": jsTk,
	"typescript": tsTk,
	"tsx":        tsTk,
	"bash":       bashTk,
	"json":       jsonTk,
	"scheme":     schemeTk,
}

// highlightSource parses code with the gotreesitter grammar named by lang
// (any linguist alias or grammar name grammars.DetectLanguageByName accepts)
// and returns HTML with `tk-*` highlight spans. ok is false when the
// language can't be resolved, fails to parse, or produces no classifiable
// spans — callers should fall back to a plain, unhighlighted `.codebody` in
// that case rather than treat it as an error.
func highlightSource(lang, code string) (string, bool) {
	if lang == "" || code == "" {
		return "", false
	}
	entry := grammars.DetectLanguageByName(lang)
	if entry == nil {
		return "", false
	}
	language := entry.Language()
	if language == nil {
		return "", false
	}
	src := []byte(code)
	tree, err := gts.NewParser(language).Parse(src)
	if err != nil || tree == nil {
		return "", false
	}
	defer tree.Release()
	root := tree.RootNode()
	if root == nil {
		return "", false
	}

	cls := tkClassifiers[entry.Name]
	var spans []tkSpan
	collectSpans(root, language, src, cls, &spans)
	if len(spans) == 0 {
		return "", false
	}
	return renderSpans(code, spans), true
}

type tkSpan struct {
	start uint32
	end   uint32
	class string
}

// collectSpans walks the tree recording a span for the first classifiable
// node on each path (mirrors mdpp's highlight_native.go collect) — once a
// node classifies, its children are not visited separately, so spans never
// overlap.
func collectSpans(n *gts.Node, lang *gts.Language, src []byte, cls *tkClassifier, spans *[]tkSpan) {
	if n == nil {
		return
	}
	if class := classify(n, lang, src, cls); class != "" && n.EndByte() > n.StartByte() {
		*spans = append(*spans, tkSpan{start: n.StartByte(), end: n.EndByte(), class: class})
		return
	}
	for i := 0; i < n.ChildCount(); i++ {
		collectSpans(n.Child(i), lang, src, cls, spans)
	}
}

func classify(n *gts.Node, lang *gts.Language, src []byte, cls *tkClassifier) string {
	nodeType := n.Type(lang)

	if cls != nil {
		if class, ok := cls.exact[nodeType]; ok {
			return class
		}
	}
	if cls == schemeTk {
		if class := classifySchemeSymbol(n, lang, nodeType, src); class != "" {
			return class
		}
	}
	if class := classifyCall(n, lang, nodeType, cls); class != "" {
		return class
	}

	text := n.Text(src)
	if cls != nil && cls.keywords[text] {
		return "tk-kw"
	}
	if cls == nil {
		return classifyGeneric(nodeType, text)
	}
	if isTkOperator(text) {
		return "tk-op"
	}
	return ""
}

// classifyCall returns tk-fn when n names a function: the declared name in a
// function/method definition, or the callee of a call expression.
func classifyCall(n *gts.Node, lang *gts.Language, nodeType string, cls *tkClassifier) string {
	if nodeType != "identifier" && nodeType != "field_identifier" && nodeType != "property_identifier" {
		return ""
	}
	parent := n.Parent()
	if parent == nil {
		return ""
	}
	parentType := parent.Type(lang)

	switch parentType {
	case "function_declaration", "method_declaration",
		"function_definition", "function_item",
		"method_definition":
		return "tk-fn"
	}

	callParent := "call_expression"
	if cls != nil {
		callParent = cls.callParent
	}
	if callParent == "" {
		return ""
	}
	if parentType == callParent && parent.Child(0) == n {
		return "tk-fn"
	}
	if parentType == "selector_expression" || parentType == "member_expression" {
		if nodeType != "field_identifier" && nodeType != "property_identifier" {
			return ""
		}
		if grandparent := parent.Parent(); grandparent != nil &&
			grandparent.Type(lang) == callParent && grandparent.Child(0) == parent {
			return "tk-fn"
		}
	}
	return ""
}

// classifySchemeSymbol classifies tree-sitter *query* syntax parsed through
// the Scheme grammar (see schemeTk's doc comment) — recognizing the parts of
// query syntax that Scheme's own reader either has no node type for (a bare
// `symbol`) or turns into an `ERROR` token (`#pred?`/`#directive!`, since
// Scheme's `#` reader syntax only covers `#t`/`#f`/`#\char`/etc, not
// arbitrary `#name` identifiers) — confirmed directly against every example
// in content/docs/queries.md, code-navigation.md, and language-injection.md.
func classifySchemeSymbol(n *gts.Node, lang *gts.Language, nodeType string, src []byte) string {
	text := n.Text(src)
	if nodeType == "ERROR" && strings.HasPrefix(text, "#") {
		// Predicate/directive name: #eq?, #match?, #set!, #any-of?, ...
		return "tk-kw"
	}
	if nodeType != "symbol" {
		return ""
	}
	switch {
	case text == ".":
		return "tk-op" // adjacency anchor
	case text == "_":
		return "tk-op" // any-node wildcard
	case strings.HasPrefix(text, "@"):
		return "tk-fn" // capture name, e.g. @func.name
	case strings.HasPrefix(text, "!"):
		return "tk-op" // negated field, e.g. !result
	case strings.HasSuffix(text, ":"):
		return "tk-op" // field constraint label, e.g. name:
	}
	if isQueryTypePosition(n, lang) {
		return "tk-ty" // grammar node-type name, e.g. function_declaration
	}
	return ""
}

// isQueryTypePosition reports whether n is the node-type symbol immediately
// following a pattern's opening `(` or `[` — the first child of its parent
// `list` node, right after the opener. That position is always a grammar
// node-type name in tree-sitter query syntax (`(function_declaration ...)`,
// `[(function_declaration) (method_declaration)]`), never a capture, field
// label, or predicate.
func isQueryTypePosition(n *gts.Node, lang *gts.Language) bool {
	parent := n.Parent()
	if parent == nil || parent.Type(lang) != "list" || parent.ChildCount() < 2 {
		return false
	}
	opener := parent.Child(0)
	if opener == nil {
		return false
	}
	switch opener.Type(lang) {
	case "(", "[":
	default:
		return false
	}
	return parent.Child(1) == n
}

// classifyGeneric is the fallback for any of gotreesitter's 206 grammars
// without a hand-tuned table above: shallow structural/text heuristics that
// still come from a real parse, not a guess.
func classifyGeneric(nodeType, text string) string {
	if strings.Contains(nodeType, "comment") {
		return "tk-cm"
	}
	if strings.Contains(nodeType, "string") {
		return "tk-str"
	}
	switch nodeType {
	case "interpreted_string_literal", "raw_string_literal", "string_literal",
		"template_string", "string_content", "escape_sequence":
		return "tk-str"
	case "int_literal", "float_literal", "imaginary_literal", "rune_literal",
		"number", "integer", "float":
		return "tk-num"
	case "type_identifier":
		return "tk-ty"
	}
	switch text {
	case "true", "false", "True", "False":
		return "tk-kw"
	case "nil", "null", "undefined", "None":
		return "tk-kw"
	}
	if genericTkKeywords[nodeType] || genericTkKeywords[text] {
		return "tk-kw"
	}
	if nodeType == "identifier" && len(text) > 0 && text[0] >= 'A' && text[0] <= 'Z' {
		return "tk-ty"
	}
	if isTkOperator(text) {
		return "tk-op"
	}
	return ""
}

var genericTkKeywords = map[string]bool{
	"keyword": true,

	"async": true, "await": true, "break": true, "case": true, "chan": true,
	"class": true, "const": true, "continue": true, "default": true, "def": true,
	"defer": true, "del": true, "elif": true, "else": true, "except": true,
	"export": true, "extends": true, "fallthrough": true, "finally": true,
	"for": true, "from": true, "func": true, "function": true, "go": true,
	"goto": true, "if": true, "import": true, "in": true, "interface": true,
	"is": true, "lambda": true, "let": true, "map": true, "new": true,
	"not": true, "or": true, "and": true, "package": true, "pass": true,
	"raise": true, "range": true, "return": true, "select": true,
	"struct": true, "switch": true, "try": true, "type": true, "var": true,
	"while": true, "with": true, "yield": true,
}

func isTkOperator(token string) bool {
	switch token {
	case "+", "-", "*", "/", "%", "=", "==", "!=", "<", ">", "<=", ">=",
		":=", "&&", "||", "!", "...", "**", "//", "+=", "-=", "*=", "/=",
		"&", "|", "^", "~", "<<", ">>", "->", "=>", "::", "?":
		return true
	default:
		return false
	}
}

// renderSpans interleaves escaped plain-text runs with `<span class="tk-*">`
// wrapped, escaped token runs, in byte order. Mirrors mdpp's
// renderHighlightedHTML but against tk-* classes.
func renderSpans(source string, spans []tkSpan) string {
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].start == spans[j].start {
			return spans[i].end < spans[j].end
		}
		return spans[i].start < spans[j].start
	})

	src := []byte(source)
	pos := uint32(0)
	end := uint32(len(src))

	var b strings.Builder
	b.Grow(len(source) * 2)

	for _, span := range spans {
		if span.start < pos || span.end > end || span.end <= pos {
			continue
		}
		if span.start > pos {
			b.WriteString(html.EscapeString(string(src[pos:span.start])))
		}
		b.WriteString(`<span class="`)
		b.WriteString(span.class)
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(string(src[span.start:span.end])))
		b.WriteString(`</span>`)
		pos = span.end
	}
	if pos < end {
		b.WriteString(html.EscapeString(string(src[pos:end])))
	}
	return b.String()
}
