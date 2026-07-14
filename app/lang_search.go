package docs

// LangSearchTokenSources returns the language names that need the catalog's
// TS badge. The server component receives names and badges as compact string
// slices; CSS preserves the established eight-color rotation by tile position.
func LangSearchTokenSources(names []string) []string {
	items := make([]string, 0, len(langTokenSourceSet))
	for _, name := range names {
		if langTokenSourceSet[name] {
			items = append(items, name)
		}
	}
	return items
}
