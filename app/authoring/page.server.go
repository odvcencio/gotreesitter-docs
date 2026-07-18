package authoring

import (
	"embed"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	"m31labs.dev/gosx/route"
	"m31labs.dev/gosx/server"
)

//go:embed page.gsx
var pageSource embed.FS

// initialGrammarJSON is grammargen's built-in CalcGrammar (grammargen.CalcGrammar,
// binary ops +-*/ with precedence/associativity, unary prefix minus,
// parenthesized expressions, integer literals) round-tripped through
// ExportGrammarJSON. It compiles sub-millisecond natively (see
// BenchmarkLRTableGeneration in the engine repo), so the first in-browser
// compile+parse pass on page load is effectively instant.
const initialGrammarJSON = `{
  "name": "calc",
  "rules": {
    "program": {
      "name": "expression",
      "type": "SYMBOL"
    },
    "expression": {
      "members": [
        {
          "content": {
            "members": [
              {
                "content": {
                  "name": "expression",
                  "type": "SYMBOL"
                },
                "name": "left",
                "type": "FIELD"
              },
              {
                "content": {
                  "type": "STRING",
                  "value": "+"
                },
                "name": "operator",
                "type": "FIELD"
              },
              {
                "content": {
                  "name": "expression",
                  "type": "SYMBOL"
                },
                "name": "right",
                "type": "FIELD"
              }
            ],
            "type": "SEQ"
          },
          "type": "PREC_LEFT",
          "value": 1
        },
        {
          "content": {
            "members": [
              {
                "content": {
                  "name": "expression",
                  "type": "SYMBOL"
                },
                "name": "left",
                "type": "FIELD"
              },
              {
                "content": {
                  "type": "STRING",
                  "value": "-"
                },
                "name": "operator",
                "type": "FIELD"
              },
              {
                "content": {
                  "name": "expression",
                  "type": "SYMBOL"
                },
                "name": "right",
                "type": "FIELD"
              }
            ],
            "type": "SEQ"
          },
          "type": "PREC_LEFT",
          "value": 1
        },
        {
          "content": {
            "members": [
              {
                "content": {
                  "name": "expression",
                  "type": "SYMBOL"
                },
                "name": "left",
                "type": "FIELD"
              },
              {
                "content": {
                  "type": "STRING",
                  "value": "*"
                },
                "name": "operator",
                "type": "FIELD"
              },
              {
                "content": {
                  "name": "expression",
                  "type": "SYMBOL"
                },
                "name": "right",
                "type": "FIELD"
              }
            ],
            "type": "SEQ"
          },
          "type": "PREC_LEFT",
          "value": 2
        },
        {
          "content": {
            "members": [
              {
                "content": {
                  "name": "expression",
                  "type": "SYMBOL"
                },
                "name": "left",
                "type": "FIELD"
              },
              {
                "content": {
                  "type": "STRING",
                  "value": "/"
                },
                "name": "operator",
                "type": "FIELD"
              },
              {
                "content": {
                  "name": "expression",
                  "type": "SYMBOL"
                },
                "name": "right",
                "type": "FIELD"
              }
            ],
            "type": "SEQ"
          },
          "type": "PREC_LEFT",
          "value": 2
        },
        {
          "content": {
            "members": [
              {
                "content": {
                  "type": "STRING",
                  "value": "-"
                },
                "name": "operator",
                "type": "FIELD"
              },
              {
                "content": {
                  "name": "expression",
                  "type": "SYMBOL"
                },
                "name": "operand",
                "type": "FIELD"
              }
            ],
            "type": "SEQ"
          },
          "type": "PREC_RIGHT",
          "value": 3
        },
        {
          "members": [
            {
              "type": "STRING",
              "value": "("
            },
            {
              "name": "expression",
              "type": "SYMBOL"
            },
            {
              "type": "STRING",
              "value": ")"
            }
          ],
          "type": "SEQ"
        },
        {
          "name": "number",
          "type": "SYMBOL"
        }
      ],
      "type": "CHOICE"
    },
    "number": {
      "content": {
        "content": {
          "type": "PATTERN",
          "value": "[0-9]"
        },
        "type": "REPEAT1"
      },
      "type": "TOKEN"
    }
  },
  "extras": [
    {
      "type": "PATTERN",
      "value": "\\s"
    }
  ],
  "conflicts": [],
  "externals": [],
  "inline": [],
  "supertypes": []
}
`

const initialSample = `1 + 2 * (3 - 4)`

func init() {
	docsapp.RegisterStaticDocsPage(
		"Grammar Authoring",
		"Edit a tree-sitter grammar.json in the browser and see it compiled and parsed live by grammargen, compiled to WebAssembly.",
		route.FileModuleOptions{
			Load: loadAuthoring,
			Metadata: func(ctx *route.RouteContext, page route.FilePage, data any) (server.Metadata, error) {
				return server.Metadata{
					Title: server.Title{Absolute: "Grammar Authoring — gotreesitter"},
					Links: []server.LinkTag{{
						Rel:  "stylesheet",
						Href: docsapp.PublicAssetURL("authoring/authoring.css"),
					}},
				}, nil
			},
		},
	)
}

func loadAuthoring(_ *route.RouteContext, _ route.FilePage) (any, error) {
	return map[string]any{
		"grammar":    initialGrammarJSON,
		"sample":     initialSample,
		"gtsVersion": docsapp.PlaygroundGTSVersion(),
		"wasmURL":    docsapp.PublicAssetURL("authoring/authoring.wasm"),
		// workerURL points at the generated Web Worker bootstrap
		// (cmd/build-authoring-wasm writes it alongside authoring-worker.wasm)
		// that actually runs the compile+parse+diagnose+highlight loop off
		// the main thread. See cmd/authoring-wasm/main_js.go's package doc.
		"workerURL": docsapp.PublicAssetURL("authoring/authoring-worker.js"),
	}, nil
}
