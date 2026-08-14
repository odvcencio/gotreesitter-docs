package main

import (
	"testing"
	"unicode/utf16"
)

func TestUTF16IndexMatchesJavaScriptStringLength(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"ascii", "package main\nfunc alpha() {}\n"},
		{"latin1", "// café au lait\nvar x = 1\n"},
		{"cjk", "// 中文注释\nvar y = 2\n"},
		{"emoji_bmp_and_astral", "// ✓ done \U0001F600\nvar z = 3\n"},
		{"astral_only", "\U0001F600\U0001F601\U0001F602"},
		{"empty", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// At every rune boundary, utf16Index must agree with the UTF-16
			// length of the same prefix — which is exactly what JavaScript
			// reports for source.slice(0, n).length.
			for byteOffset := range tc.source {
				want := len(utf16.Encode([]rune(tc.source[:byteOffset])))
				if got := utf16Index(tc.source, uint32(byteOffset)); got != want {
					t.Fatalf("utf16Index(%q, %d) = %d, want %d", tc.name, byteOffset, got, want)
				}
			}
			// And at the very end of the string.
			want := len(utf16.Encode([]rune(tc.source)))
			if got := utf16Index(tc.source, uint32(len(tc.source))); got != want {
				t.Fatalf("utf16Index(%q, len) = %d, want %d", tc.name, got, want)
			}
		})
	}
}

func TestUTF16IndexClampsOutOfRangeOffsets(t *testing.T) {
	const source = "abc"
	if got := utf16Index(source, 99); got != 3 {
		t.Errorf("past-end offset = %d, want 3", got)
	}
	if got := utf16Index(source, 0); got != 0 {
		t.Errorf("zero offset = %d, want 0", got)
	}
	if got := utf16Index("", 5); got != 0 {
		t.Errorf("empty source = %d, want 0", got)
	}
}

// An astral rune must advance the index by two units, or a selection after an
// emoji lands one character short for every emoji before it.
func TestUTF16IndexCountsAstralRunesAsSurrogatePairs(t *testing.T) {
	const source = "\U0001F600x"
	if got := utf16Index(source, uint32(len("\U0001F600"))); got != 2 {
		t.Errorf("after astral rune = %d, want 2", got)
	}
}
