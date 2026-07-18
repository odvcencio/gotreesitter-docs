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
// Known residual gap this shim does NOT paper over (left visible
// deliberately): grammargen.EmitC also branches its `ts_lex_modes` array's
// element type on the grammar's own LanguageVersion — `TSLexMode` (the
// older, 2-field struct) for ABI < 15, `TSLexerMode` (the newer, 3-field
// struct this header's `TSLanguage.lex_modes` field is actually typed with)
// for ABI >= 15. For an ABI-14-shaped grammar (e.g. this page's "calc" seed)
// that produces a real `-Wincompatible-pointer-types` WARNING assigning
// `.lex_modes = ts_lex_modes` — a genuine, pre-existing mismatch between
// grammargen's own emitted element type and this header's field type for
// low-ABI grammars, not something introduced by gotreesitter-docs. It is a
// warning under C's default (non -Werror) rules, not a hard error, so it
// does not fail cmd/verify-authoring-browser's syntax-only check — but it is
// real and worth flagging upstream. Confirmed absent for ABI-15 grammars
// (e.g. "json"): those compile clean, zero warnings, with only this one
// shim applied.
#ifndef GOTREESITTER_DOCS_VERIFY_AUTHORING_COMPAT_SHIM_H_
#define GOTREESITTER_DOCS_VERIFY_AUTHORING_COMPAT_SHIM_H_

typedef TSMapSlice TSFieldMapSlice;

#endif
