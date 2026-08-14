package playgroundengine

import "testing"

// TestTypeScriptShiftOperatorParsesInPlayground is the end-to-end payoff of the
// gotreesitter v0.50.0 fix. Before it, the TypeScript and TSX grammars split a
// signed right-shift whose right operand was parenthesised into two generic
// closers, so a visitor pasting ordinary bit-twiddling code into the playground
// got an error box. This pins the docs site to a gotreesitter that parses it.
func TestTypeScriptShiftOperatorParsesInPlayground(t *testing.T) {
	sources := []struct {
		name   string
		source string
	}{
		{"paren_rhs", "const x = a >> (b);\n"},
		{"indexed_both", "indices[i] = (src[i >> 3] >> (i & 7)) & 1;\n"},
		{"unparenthesised_control", "const y = a >> b;\n"},
		{"nested_generics_still_parse", "let v: Array<Array<string>> = [];\n"},
	}

	for _, language := range []string{"typescript", "tsx"} {
		for _, tc := range sources {
			t.Run(language+"/"+tc.name, func(t *testing.T) {
				result := Parse(tc.source, "", language, false)
				if result.ParseError != "" {
					t.Fatalf("parse error: %s\nsource: %s", result.ParseError, tc.source)
				}
				if result.HasErrors {
					t.Fatalf("tree contains ERROR nodes\nsource: %s", tc.source)
				}
				if len(result.TreeRows) == 0 {
					t.Fatalf("empty tree\nsource: %s", tc.source)
				}
			})
		}
	}
}
