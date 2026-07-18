package authoringengine

import (
	"context"
	"strings"
	"testing"
	"time"
)

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

// trivialGrammarJSON has no ambiguity at all — a single fixed-string rule —
// so it is the "0 conflicts" control case for TestCompileConflictsAndHighlights.
const trivialGrammarJSON = `{
  "name": "trivial",
  "rules": {
    "program": {
      "type": "STRING",
      "value": "hello"
    }
  },
  "extras": [],
  "conflicts": [],
  "externals": [],
  "inline": [],
  "supertypes": []
}`

func TestCompileConflictsAndHighlights(t *testing.T) {
	result := Compile(calcGrammarJSON, "1 + 2 * (3 - 4)", true)
	if result.GenerateError != "" {
		t.Fatalf("GenerateError: %s", result.GenerateError)
	}
	// Calc's precedence/associativity annotations resolve every state where
	// the LR builder finds more than one candidate action; grammargen still
	// reports each one as a diagnosable conflict (this is exactly the
	// "author sees exactly what's ambiguous" payoff — even a well-formed
	// grammar has real conflicts, all auto-resolved).
	if len(result.Conflicts) == 0 {
		t.Fatal("expected calc's precedence-resolved shift/reduce conflicts to be reported")
	}
	for _, c := range result.Conflicts {
		if c.Kind != "shift/reduce" && c.Kind != "reduce/reduce" {
			t.Errorf("conflict Kind: got %q, want shift/reduce or reduce/reduce", c.Kind)
		}
		if c.Resolution == "" {
			t.Error("conflict Resolution is empty")
		}
		if c.Description == "" {
			t.Error("conflict Description is empty")
		}
	}

	if len(result.Highlights) == 0 {
		t.Fatalf("expected highlight spans for the operators in %q (notice: %q)", "1 + 2 * (3 - 4)", result.HighlightNotice)
	}
	for _, h := range result.Highlights {
		if h.EndByte <= h.StartByte {
			t.Errorf("highlight span has non-positive width: %+v", h)
		}
		if h.Capture == "" {
			t.Error("highlight span has empty Capture")
		}
	}
}

func TestCompileTrivialGrammarHasNoConflicts(t *testing.T) {
	result := Compile(trivialGrammarJSON, "hello", true)
	if result.ImportError != "" || result.GenerateError != "" || result.ParseError != "" {
		t.Fatalf("unexpected errors: import=%q generate=%q parse=%q", result.ImportError, result.GenerateError, result.ParseError)
	}
	if len(result.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts for an unambiguous grammar, got %d", len(result.Conflicts))
	}
}

func TestCompileWithContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := CompileWithContext(ctx, calcGrammarJSON, "1+2", true)
	if !result.TimedOut {
		t.Error("expected TimedOut=true for a pre-cancelled context")
	}
	if result.GenerateError == "" {
		t.Error("expected a GenerateError describing the cancellation")
	}
	if result.ImportError != "" {
		t.Errorf("ImportError should be empty (import happens before the context is consulted): %q", result.ImportError)
	}
}

func TestCompileWithContextDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)
	result := CompileWithContext(ctx, calcGrammarJSON, "1+2", true)
	if !result.TimedOut {
		t.Error("expected TimedOut=true once the deadline has already passed")
	}
	if !strings.Contains(result.GenerateError, "time budget") {
		t.Errorf("GenerateError should mention the time budget: %q", result.GenerateError)
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
