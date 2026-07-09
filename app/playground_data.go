package docs

// Playground sample data — Island 1 (design/PHASE-B-NOTES.md: "samples{ go,
// json, python }: each sample: code tokens + tree + stats + presets" —
// content as data, not hand-written markup). Ported directly from
// design/GoTreeSitter-Docs.html's own `samples` / `sampleCode` / `presets`
// JS object literals (search that file for "samples = {" and "sampleCode =
// {") so the playground shows exactly the same three worked examples the
// design specified.

// codeTok is one rendered token in a sample's source code panel.
// Newline tokens render as <br>; tokens with an empty Class render as plain
// text (design's own buildCode special-cased "sp"/space runs the same way —
// no wrapper span needed for something no CSS rule ever targets).
type codeTok struct {
	Class   string // tk-kw/tk-fn/tk-id/tk-ty/tk-str/tk-num/tk-op/tk-pn ("" = plain text)
	Text    string
	Newline bool
}

func kw(s string) codeTok  { return codeTok{Class: "tk-kw", Text: s} }
func fn(s string) codeTok  { return codeTok{Class: "tk-fn", Text: s} }
func id(s string) codeTok  { return codeTok{Class: "tk-id", Text: s} }
func ty(s string) codeTok  { return codeTok{Class: "tk-ty", Text: s} }
func str(s string) codeTok { return codeTok{Class: "tk-str", Text: s} }
func num(s string) codeTok { return codeTok{Class: "tk-num", Text: s} }
func op(s string) codeTok  { return codeTok{Class: "tk-op", Text: s} }
func pn(s string) codeTok  { return codeTok{Class: "tk-pn", Text: s} }
func sp(s string) codeTok  { return codeTok{Text: s} }

var nl = codeTok{Newline: true}

// treeRow is one syntax-tree node in a sample's tree panel — mirrors the
// design's `[type, field, text, children, mark]` tuple shape exactly.
// mark is "reused", "edited", or "" (design/PHASE-B-NOTES.md's "per-node
// mark (reused/edited), applied ONLY when edited === true").
type treeRow struct {
	Type     string
	Field    string
	Text     string
	Mark     string
	Children []treeRow
}

// playgroundSample bundles one sample language's full playground state:
// tokenized source, syntax tree, parse stats, and S-expression query
// presets — see design/GoTreeSitter-Docs.html's samples/sampleCode/presets.
type playgroundSample struct {
	ID    string
	Label string

	StatTime   string
	StatNodes  string
	StatBytes  string
	StatAllocs string

	Code []codeTok
	Tree treeRow

	Presets []string
}

var playgroundSamples = []playgroundSample{
	{
		ID:    "go",
		Label: "Go",

		StatTime: "1.54 ms", StatNodes: "6,014", StatBytes: "19.3 KB", StatAllocs: "7",

		Code: []codeTok{
			kw("package"), sp(" "), id("main"), nl, nl,
			kw("func"), sp(" "), fn("greet"), pn("("), id("name"), sp(" "), ty("string"), pn(") "), ty("string"), sp(" "), pn("{"), nl,
			sp("    "), kw("return"), sp(" "), str(`"hey, "`), sp(" "), op("+"), sp(" "), id("name"), nl,
			pn("}"),
		},

		Tree: treeRow{Type: "source_file", Children: []treeRow{
			{Type: "package_clause", Mark: "reused", Children: []treeRow{
				{Type: "package_identifier", Text: "main"},
			}},
			{Type: "function_declaration", Children: []treeRow{
				{Type: "identifier", Field: "name", Text: "greet", Mark: "reused"},
				{Type: "parameter_list", Field: "parameters", Mark: "reused", Children: []treeRow{
					{Type: "parameter_declaration", Children: []treeRow{
						{Type: "identifier", Field: "name", Text: "name"},
						{Type: "type_identifier", Field: "type", Text: "string"},
					}},
				}},
				{Type: "type_identifier", Field: "result", Text: "string"},
				{Type: "block", Field: "body", Children: []treeRow{
					{Type: "return_statement", Children: []treeRow{
						{Type: "binary_expression", Children: []treeRow{
							{Type: "interpreted_string_literal", Text: `"hey, "`, Mark: "edited"},
							{Type: "identifier", Text: "name"},
						}},
					}},
				}},
			}},
		}},

		Presets: []string{
			"(identifier) @name",
			"(function_declaration name: (identifier) @fn)",
			"(interpreted_string_literal) @str",
		},
	},
	{
		ID:    "json",
		Label: "JSON",

		StatTime: "0.31 ms", StatNodes: "11", StatBytes: "34 B", StatAllocs: "2",

		Code: []codeTok{
			pn("{"), nl,
			sp("  "), str(`"name"`), pn(": "), str(`"fable"`), pn(","), nl,
			sp("  "), str(`"port"`), pn(": "), num("9090"), nl,
			pn("}"),
		},

		Tree: treeRow{Type: "document", Children: []treeRow{
			{Type: "object", Children: []treeRow{
				{Type: "pair", Children: []treeRow{
					{Type: "string", Field: "key", Text: `"name"`, Mark: "reused"},
					{Type: "string", Field: "value", Text: `"fable"`},
				}},
				{Type: "pair", Children: []treeRow{
					{Type: "string", Field: "key", Text: `"port"`, Mark: "reused"},
					{Type: "number", Field: "value", Text: "9090", Mark: "edited"},
				}},
			}},
		}},

		Presets: []string{
			"(pair key: (string) @key)",
			"(number) @num",
			"(string) @str",
		},
	},
	{
		ID:    "python",
		Label: "Python",

		StatTime: "0.44 ms", StatNodes: "14", StatBytes: "41 B", StatAllocs: "3",

		Code: []codeTok{
			kw("def"), sp(" "), fn("greet"), pn("("), id("name"), pn("):"), nl,
			sp("    "), kw("return"), sp(" "), str(`f"hey {name}"`),
		},

		Tree: treeRow{Type: "module", Children: []treeRow{
			{Type: "function_definition", Children: []treeRow{
				{Type: "identifier", Field: "name", Text: "greet", Mark: "reused"},
				{Type: "parameters", Field: "parameters", Mark: "reused", Children: []treeRow{
					{Type: "identifier", Text: "name"},
				}},
				{Type: "block", Field: "body", Children: []treeRow{
					{Type: "return_statement", Children: []treeRow{
						{Type: "string", Text: `f"hey {name}"`, Mark: "edited"},
					}},
				}},
			}},
		}},

		Presets: []string{
			"(identifier) @name",
			"(function_definition) @def",
			"(string) @str",
		},
	},
}
