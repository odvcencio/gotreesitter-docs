// Compatibility shim — NOT part of upstream tree-sitter. Bridges one real
// naming gap between grammargen.EmitC's emitted parser.c and the vendored,
// unmodified tree_sitter/parser.h in this same directory.
//
// Background: gotreesitter's grammargen.EmitC (github.com/odvcencio/gotreesitter,
// grammargen/codegen_c.go) always emits a `TSLanguage.field_map_slices` array
// typed `TSFieldMapSlice` — an older tree-sitter type name. The vendored
// parser.h here (a real, current upstream header — see its own top-of-file
// comment) renamed that type to `TSMapSlice` some time ago (its own comment
// reads "Used to index the field and supertype maps"), and the
// `TSLanguage.field_map_slices` field itself is typed `const TSMapSlice *`.
// Without this alias, `TSFieldMapSlice` is simply an undeclared identifier —
// a hard compile error, not a mere naming-convention warning. `TSMapSlice`'s
// two fields (`index`, `length`) are exactly what EmitC's own struct-literal
// initializers write (`.index = ..., .length = ...`), so the alias is a
// faithful, zero-risk bridge, not a workaround for any real shape mismatch.
//
// In gotreesitter v0.57.0, EmitC keeps the two-field TSLexMode table for
// ABI < 15 and initializes TSLanguage.lex_modes through const void *. The
// runtime reads that table with the ABI-14 stride. ABI-15 output uses the
// three-field TSLexerMode table. This resolves the older initializer warning
// without changing either table layout.
#ifndef GOTREESITTER_DOCS_VERIFY_AUTHORING_COMPAT_SHIM_H_
#define GOTREESITTER_DOCS_VERIFY_AUTHORING_COMPAT_SHIM_H_

typedef TSMapSlice TSFieldMapSlice;

#endif
