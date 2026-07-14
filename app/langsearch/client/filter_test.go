package main

import "testing"

func TestLanguageMatchesQuery(t *testing.T) {
	tests := []struct {
		language string
		query    string
		want     bool
	}{
		{language: "Go", query: "go", want: true},
		{language: "gomod", query: "GO", want: true},
		{language: "python", query: "go", want: false},
		{language: "typescript", query: "", want: true},
	}
	for _, test := range tests {
		if got := languageMatchesQuery(test.language, test.query); got != test.want {
			t.Errorf("languageMatchesQuery(%q, %q) = %t, want %t", test.language, test.query, got, test.want)
		}
	}
}
