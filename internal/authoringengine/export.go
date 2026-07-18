package authoringengine

import (
	"context"
	"fmt"
	"go/format"
	"regexp"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

// ExportFormat is one of the three download targets Phase 3's export
// buttons (#ag-export-go/#ag-export-c/#ag-export-json) offer — see
// cmd/authoring-worker-wasm's "export" op and cmd/authoring-wasm's
// onExportClick.
type ExportFormat string

const (
	ExportFormatGo   ExportFormat = "go"
	ExportFormatC    ExportFormat = "c"
	ExportFormatJSON ExportFormat = "json"
)

// grammargenImportPath is dot-imported into every "go" export — see ExportGrammar's
// doc for why a dot import (not a qualified one) is required for
// grammargen.EmitGrammarGo's output to compile standalone.
const grammargenImportPath = "github.com/odvcencio/gotreesitter/grammargen"

// ImportAndGenerateWithContext runs just the grammar.json -> Grammar ->
// Language prefix of CompileWithContextArtifacts (skipping diagnostics,
// highlighting, and the sample parse) — the minimum work an export needs
// when no cached compile is available to reuse. Like
// CompileWithContextArtifacts, it never touches the filesystem or network.
//
// Used by cmd/authoring-worker-wasm (a cache miss on export — the currently
// authored grammar differs from whatever was last compiled) and by
// cmd/authoring-wasm's main-thread fallback (no worker available at all, so
// there is no cache to check in the first place).
func ImportAndGenerateWithContext(ctx context.Context, grammarJSON string) (*grammargen.Grammar, *gts.Language, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(grammarJSON) > MaxGrammarBytes {
		return nil, nil, fmt.Errorf("grammar.json must be %d bytes or smaller", MaxGrammarBytes)
	}
	g, err := grammargen.ImportGrammarJSON([]byte(grammarJSON))
	if err != nil {
		return nil, nil, fmt.Errorf("import grammar.json: %w", err)
	}
	lang, err := grammargen.GenerateLanguageWithContext(ctx, g)
	if err != nil {
		return nil, nil, fmt.Errorf("generate language: %w", err)
	}
	if lang == nil {
		return nil, nil, fmt.Errorf("grammar compiled to a nil language")
	}
	return g, lang, nil
}

// ExportGrammar renders one of the three export formats from an
// already-compiled grammar. lang is optional: when non-nil (the caller
// already has a cached compile — see CompileWithContextArtifacts and
// cmd/authoring-worker-wasm's cache) format "c" reuses it directly via
// grammargen.EmitC instead of grammargen.GenerateC, which would otherwise
// pay for a second, full LR-table generation pass purely to render C —
// exactly the "do NOT recompile on export" requirement a heavy base like Go
// (~5-13s to generate, depending on machine) needs. Formats "go" and "json"
// only ever need grammar, never lang.
//
// name is used to derive the download filename (and, for "go", the package/
// function names); it is independent of grammar.Name so a caller can use
// whatever the author currently has typed in the rename field even if it
// hasn't been recompiled yet. An empty name falls back to grammar.Name, then
// to "grammar".
func ExportGrammar(grammar *grammargen.Grammar, lang *gts.Language, format ExportFormat, name, pkgName string) (filename, content string, err error) {
	if grammar == nil {
		return "", "", fmt.Errorf("export %s: no compiled grammar available", format)
	}
	stem := sanitizeFilenameStem(name)
	if stem == "" {
		stem = sanitizeFilenameStem(grammar.Name)
	}
	if stem == "" {
		stem = "grammar"
	}

	switch format {
	case ExportFormatJSON:
		data, exportErr := grammargen.ExportGrammarJSON(grammar)
		if exportErr != nil {
			return "", "", fmt.Errorf("export grammar.json: %w", exportErr)
		}
		return stem + ".grammar.json", string(data), nil

	case ExportFormatGo:
		pkg := sanitizePackageName(pkgName)
		if pkg == "" {
			pkg = sanitizePackageName(stem)
		}
		if pkg == "" {
			pkg = "authored"
		}
		// "Authored" prefix (not just exportedIdentifier(stem)+"Grammar") is
		// deliberate, not decorative: the file dot-imports grammargen (see
		// injectDotImport), which itself exports ~20 built-in
		// "func XGrammar() *Grammar" constructors (CalcGrammar, GoGrammar,
		// JSONGrammar, ...) — precisely the names an author inheriting from
		// one of those bases (e.g. this page's own default "calc" seed) would
		// otherwise collide with, producing a top-level redeclaration that
		// fails to build. Verified empirically: funcName="CalcGrammar" with a
		// dot import fails with "CalcGrammar already declared through
		// dot-import of package grammargen". No built-in grammargen
		// constructor starts with "Authored".
		funcName := "Authored" + exportedIdentifier(stem) + "Grammar"
		data, emitErr := grammargen.EmitGrammarGo(grammar, pkg, funcName)
		if emitErr != nil {
			return "", "", fmt.Errorf("emit grammar go source: %w", emitErr)
		}
		return stem + ".go", string(injectDotImport(data, pkg)), nil

	case ExportFormatC:
		if lang != nil {
			data, emitErr := grammargen.EmitC(grammar.Name, lang)
			if emitErr != nil {
				return "", "", fmt.Errorf("emit parser.c: %w", emitErr)
			}
			return "parser.c", data, nil
		}
		data, genErr := grammargen.GenerateC(grammar)
		if genErr != nil {
			return "", "", fmt.Errorf("generate parser.c: %w", genErr)
		}
		return "parser.c", data, nil

	default:
		return "", "", fmt.Errorf("unknown export format %q", format)
	}
}

// packageLineRe finds the end of EmitGrammarGo's leading "package X\n" line
// so injectDotImport can splice an import block in right after it, without
// depending on the exact blank-line spacing gofmt happened to produce.
var packageLineRe = regexp.MustCompile(`(?m)^package\s+\S+\n`)

// injectDotImport turns grammargen.EmitGrammarGo's raw output into a
// standalone, compilable Go file. EmitGrammarGo intentionally emits
// unqualified calls (NewGrammar(...), Str(...), Seq(...), ...) — the exact
// grammargen DSL surface — with no import statement at all, because its own
// use case (see the engine's own generated grammar sources) is being pasted
// directly inside package grammargen itself. A file downloaded from the
// browser is not inside that package, so those identifiers would otherwise
// be undefined; the only way to keep them unqualified in a different package
// is a dot import of grammargen. This is the one respect in which the
// exported ".go" is closer to "portable grammar-construction source" than a
// literal standalone parser: running it still requires a caller to invoke
// grammargen.GenerateLanguage on the returned *Grammar to get a working
// Language/parser (grammargen does not emit LR tables as Go source the way
// GenerateC/EmitC do for C).
func injectDotImport(source []byte, pkgName string) []byte {
	loc := packageLineRe.FindIndex(source)
	if loc == nil {
		return source
	}
	insertAt := loc[1]
	imp := []byte("\nimport . \"" + grammargenImportPath + "\"\n")
	out := make([]byte, 0, len(source)+len(imp))
	out = append(out, source[:insertAt]...)
	out = append(out, imp...)
	out = append(out, source[insertAt:]...)
	if formatted, err := format.Source(out); err == nil {
		return formatted
	}
	return out
}

// nonIdentRunRe matches any run of characters that cannot appear in a Go
// identifier.
var nonIdentRunRe = regexp.MustCompile(`[^A-Za-z0-9_]+`)

// sanitizeFilenameStem turns an arbitrary author-supplied string (the
// #ag-name rename field, or a grammar's own "name") into a filesystem-safe
// filename stem: letters, digits, "-", "_" only, trimmed of leading/trailing
// separators. Returns "" if nothing safe remains, so callers can fall back.
func sanitizeFilenameStem(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-_")
}

// sanitizeIdentifier turns s into a valid lowercase Go identifier fragment
// (used for the exported .go file's package name). Returns "" if nothing
// safe remains.
func sanitizeIdentifier(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonIdentRunRe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return ""
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "g" + s
	}
	return s
}

// goKeywords are the identifiers sanitizePackageName must never return bare
// — they are syntax errors in a "package X" clause, not just unconventional
// names. ("go" is a real case, not a hypothetical one: it is one of this
// page's own shipped base grammars, so exporting the unmodified "go" base
// with no rename would otherwise emit "package go", which fails to parse.
// Verified empirically before this guard was added.)
var goKeywords = map[string]bool{
	"break": true, "default": true, "func": true, "interface": true, "select": true,
	"case": true, "defer": true, "go": true, "map": true, "struct": true,
	"chan": true, "else": true, "goto": true, "package": true, "switch": true,
	"const": true, "fallthrough": true, "if": true, "range": true, "type": true,
	"continue": true, "for": true, "import": true, "return": true, "var": true,
}

// sanitizePackageName is sanitizeIdentifier plus a Go-keyword guard — only
// needed for the exported .go file's own "package X" clause (a package name
// must be a valid identifier and cannot be a keyword); exportedIdentifier's
// capitalized fragments never collide with a keyword (Go keywords are always
// lowercase), so it does not need this guard.
func sanitizePackageName(s string) string {
	id := sanitizeIdentifier(s)
	if id == "" {
		return ""
	}
	if goKeywords[id] {
		id += "_grammar"
	}
	return id
}

// exportedIdentifier turns s into an exported (capitalized) Go identifier
// fragment, e.g. "my-dsl" / "my_dsl" -> "MyDsl" — used for the exported .go
// file's generator function name ("<Name>Grammar").
func exportedIdentifier(s string) string {
	id := sanitizeIdentifier(s)
	if id == "" {
		return "Grammar"
	}
	parts := strings.Split(id, "_")
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	out := b.String()
	if out == "" {
		return "Grammar"
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = "G" + out
	}
	return out
}
