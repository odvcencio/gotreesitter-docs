package playgroundengine

import "testing"

// TestTreeRowsCarryByteSpans pins the byte offsets the playground needs to link
// a syntax tree row back to the source it came from. Row.Range is a display
// string built from points, so it cannot drive a text selection.
func TestTreeRowsCarryByteSpans(t *testing.T) {
	const source = "package main\nfunc alpha() {}\n"

	result := Parse(source, "", "go", false)
	if result.ParseError != "" {
		t.Fatalf("parse error: %q", result.ParseError)
	}
	if len(result.TreeRows) == 0 {
		t.Fatal("expected syntax tree rows")
	}

	// The root row must span the whole source.
	root := result.TreeRows[0]
	if root.StartByte != 0 {
		t.Errorf("root StartByte = %d, want 0", root.StartByte)
	}
	if int(root.EndByte) != len(source) {
		t.Errorf("root EndByte = %d, want %d", root.EndByte, len(source))
	}

	// Every row must carry a well-formed span inside the source, and the text
	// it slices must be non-empty for any row that is not zero-width.
	for i, row := range result.TreeRows {
		if row.EndByte < row.StartByte {
			t.Errorf("row %d (%s): EndByte %d < StartByte %d", i, row.Type, row.EndByte, row.StartByte)
		}
		if int(row.EndByte) > len(source) {
			t.Errorf("row %d (%s): EndByte %d past source length %d", i, row.Type, row.EndByte, len(source))
		}
	}

	// At least one row must name the declared function and slice back to it.
	found := false
	for _, row := range result.TreeRows {
		if row.Type != "identifier" {
			continue
		}
		if source[row.StartByte:row.EndByte] == "alpha" {
			found = true
			break
		}
	}
	if !found {
		t.Error("no identifier row sliced back to \"alpha\"")
	}
}
