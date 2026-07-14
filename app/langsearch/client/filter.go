package main

import "strings"

func languageMatchesQuery(language, query string) bool {
	return strings.Contains(strings.ToLower(language), strings.ToLower(query))
}
