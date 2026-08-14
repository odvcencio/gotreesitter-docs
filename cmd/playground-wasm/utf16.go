package main

// This file carries no build tag on purpose. utf16Index is pure Go with no DOM
// dependency, and it is the one piece of the tree-to-source linking that can be
// silently wrong, so it lives where a normal `go test` can reach it.

// utf16Index converts a byte offset into source into the UTF-16 code-unit index
// the DOM addresses text by. gotreesitter reports node spans in bytes, while
// textarea.setSelectionRange and String.prototype.length count UTF-16 units, so
// a single multi-byte rune earlier in the buffer is enough to make a byte
// offset select the wrong text. Runes outside the Basic Multilingual Plane
// occupy two units, which is why the surrogate case is counted separately.
func utf16Index(source string, byteOffset uint32) int {
	limit := int(byteOffset)
	if limit > len(source) {
		limit = len(source)
	}
	if limit < 0 {
		return 0
	}
	units := 0
	for _, r := range source[:limit] {
		if r > 0xFFFF {
			units += 2
			continue
		}
		units++
	}
	return units
}
