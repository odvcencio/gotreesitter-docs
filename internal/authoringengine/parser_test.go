package authoringengine

import "testing"

// calcGrammarJSON is grammargen.CalcGrammar() exported through
// ExportGrammarJSON — the same seed embedded in app/authoring/page.server.go
// (initialGrammarJSON). Duplicated here (rather than importing the app
// package, which would be a layering inversion) so this test exercises the
// exact author -> compile -> parse loop the browser engine runs, natively
// and fast, without requiring a wasm build or a browser.
const calcGrammarJSON = `{
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

func TestCompileSeedGrammar(t *testing.T) {
	result := Compile(calcGrammarJSON, "1 + 2 * (3 - 4)", true)
	if result.ImportError != "" {
		t.Fatalf("ImportError: %s", result.ImportError)
	}
	if result.GenerateError != "" {
		t.Fatalf("GenerateError: %s", result.GenerateError)
	}
	if result.ParseError != "" {
		t.Fatalf("ParseError: %s", result.ParseError)
	}
	if result.HasErrors {
		t.Errorf("expected a clean parse, tree reported errors")
	}
	if result.GrammarName != "calc" {
		t.Errorf("GrammarName: got %q, want %q", result.GrammarName, "calc")
	}
	if len(result.TreeRows) == 0 {
		t.Fatal("expected non-empty TreeRows")
	}
	if result.NodeCount == 0 {
		t.Error("expected NodeCount > 0")
	}

	foundExpression, foundNumber := false, false
	for _, row := range result.TreeRows {
		switch row.Type {
		case "expression":
			foundExpression = true
		case "number":
			foundNumber = true
		}
	}
	if !foundExpression {
		t.Error("tree rows do not contain an expression node")
	}
	if !foundNumber {
		t.Error("tree rows do not contain a number node")
	}
}

func TestCompileRejectsBadGrammarJSON(t *testing.T) {
	result := Compile("{not valid json", "1", true)
	if result.ImportError == "" {
		t.Fatal("expected ImportError for invalid grammar.json")
	}
}

func TestCompileOversizedGrammarIsRejected(t *testing.T) {
	oversized := make([]byte, MaxGrammarBytes+1)
	for i := range oversized {
		oversized[i] = ' '
	}
	result := Compile(string(oversized), "1", true)
	if result.ImportError == "" {
		t.Fatal("expected ImportError for oversized grammar.json")
	}
}
