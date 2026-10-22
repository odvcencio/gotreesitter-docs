package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Rule identifiers reported by the gate.
const (
	RuleScriptFile     = "authored-script-file"
	RuleScriptTag      = "script-tag"
	RuleJavaScriptURL  = "javascript-url"
	RuleInlineHandler  = "inline-handler"
	RuleScriptElement  = "script-element"
	RuleJSInGoString   = "js-in-go-string"
	RuleSyscallJS      = "syscall-js-dom"
	RuleStaleAllowlist = "stale-allowlist-entry"
)

// Violation is one finding. Line is 0 when the rule applies to a whole file.
type Violation struct {
	Path   string
	Rule   string
	Detail string
	Line   int
}

// Entry is one allowlist row: path, rule, reason and the work that removes it.
type Entry struct {
	Path, Rule, Reason, RemovedBy string
}

// scopePatterns are the path patterns the gate covers. A pattern ending in "/"
// matches a directory prefix; others use path.Match against the whole path or
// the leading path elements.
var scopePatterns = []string{"app/", "public/", "main.go", "cmd/*-wasm/"}

// ScopeRoots are the git pathspecs that cover the same scope.
func ScopeRoots() []string {
	return []string{"app", "public", "main.go", "cmd/*-wasm/*"}
}

// ListFiles lists tracked files and untracked-but-not-ignored files in scope,
// so generated, gitignored outputs are never scanned.
func ListFiles(root string) ([]string, error) {
	args := append([]string{"-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--"}, ScopeRoots()...)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git ls-files: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var files []string
	for _, name := range strings.Split(stdout.String(), "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	return InScope(files), nil
}

// InScope keeps the files the gate covers, sorted and without duplicates.
func InScope(files []string) []string {
	seen := map[string]bool{}
	var kept []string
	for _, file := range files {
		file = path.Clean(filepath.ToSlash(file))
		if seen[file] || !inScope(file) {
			continue
		}
		seen[file] = true
		kept = append(kept, file)
	}
	sort.Strings(kept)
	return kept
}

func inScope(file string) bool {
	for _, pattern := range scopePatterns {
		if strings.HasSuffix(pattern, "/") {
			dir := strings.TrimSuffix(pattern, "/")
			if strings.Contains(dir, "*") {
				parts := strings.Split(file, "/")
				depth := strings.Count(dir, "/") + 1
				if len(parts) > depth {
					if ok, _ := path.Match(dir, strings.Join(parts[:depth], "/")); ok {
						return true
					}
				}
			} else if strings.HasPrefix(file, pattern) {
				return true
			}
			continue
		}
		if file == pattern {
			return true
		}
	}
	return false
}

var (
	scriptTag     = regexp.MustCompile(`(?i)<script\b([^>]*)>`)
	jsonScript    = regexp.MustCompile(`(?i)\btype\s*=\s*["']?\s*application/(?:ld\+)?json\b`)
	javascriptURL = regexp.MustCompile(`(?i)javascript:`)
	inlineHandler = regexp.MustCompile(`(?i)\son[a-z]+\s*=\s*["']`)
	scriptElement = regexp.MustCompile(`\bEl\(\s*"script"`)
	jsInString    = regexp.MustCompile(`importScripts\(|new Worker\(|WebAssembly\.instantiate\(`)
)

var scriptExtensions = map[string]bool{
	".js": true, ".mjs": true, ".cjs": true, ".ts": true, ".jsx": true, ".tsx": true,
}

// Scan applies every rule to files (slash paths relative to root).
func Scan(root string, files []string) ([]Violation, error) {
	var violations []Violation
	for _, file := range files {
		base := path.Base(file)
		ext := path.Ext(base)
		switch {
		case scriptExtensions[ext]:
			if strings.Contains(base, ".test.") || strings.Contains(base, "_test.") {
				continue
			}
			violations = append(violations, Violation{Path: file, Rule: RuleScriptFile, Detail: "authored script file"})
		case ext == ".gsx":
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
			if err != nil {
				return nil, err
			}
			violations = append(violations, scanText(file, string(data), 0, false)...)
		case ext == ".go":
			if strings.HasSuffix(base, "_test.go") {
				continue
			}
			found, err := scanGo(root, file)
			if err != nil {
				return nil, err
			}
			violations = append(violations, found...)
		}
	}
	sort.SliceStable(violations, func(i, j int) bool {
		a, b := violations[i], violations[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Rule < b.Rule
	})
	return violations, nil
}

// scanText applies the markup rules to text. baseLine is the line of the
// text's first character (0 when unknown); inGoString enables the rule that
// flags a JavaScript program kept in a Go string.
func scanText(file, text string, baseLine int, inGoString bool) []Violation {
	var out []Violation
	lineOf := func(offset int) int {
		if baseLine == 0 {
			return 1 + strings.Count(text[:offset], "\n")
		}
		return baseLine + strings.Count(text[:offset], "\n")
	}
	for _, match := range scriptTag.FindAllStringSubmatchIndex(text, -1) {
		attrs := text[match[2]:match[3]]
		if jsonScript.MatchString(attrs) {
			continue
		}
		out = append(out, Violation{Path: file, Rule: RuleScriptTag, Detail: "<script> element", Line: lineOf(match[0])})
	}
	for _, match := range javascriptURL.FindAllStringIndex(text, -1) {
		out = append(out, Violation{Path: file, Rule: RuleJavaScriptURL, Detail: "javascript: URL", Line: lineOf(match[0])})
	}
	for _, match := range inlineHandler.FindAllStringIndex(text, -1) {
		out = append(out, Violation{Path: file, Rule: RuleInlineHandler, Detail: "inline event handler attribute", Line: lineOf(match[0])})
	}
	if inGoString {
		for _, match := range jsInString.FindAllStringIndex(text, -1) {
			out = append(out, Violation{Path: file, Rule: RuleJSInGoString, Detail: "JavaScript kept in a Go string: " + strings.TrimSuffix(text[match[0]:match[1]], "("), Line: lineOf(match[0])})
		}
	}
	return out
}

func scanGo(root, file string) ([]Violation, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return nil, err
	}
	var out []Violation

	// Rule 3d: hand-written DOM access.
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, data, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	for _, spec := range parsed.Imports {
		if spec.Path.Value == `"syscall/js"` {
			out = append(out, Violation{Path: file, Rule: RuleSyscallJS, Detail: `import "syscall/js"`, Line: fset.Position(spec.Pos()).Line})
		}
	}

	// Rule 3b: El("script", ...) on the raw source.
	for i, line := range strings.Split(string(data), "\n") {
		if scriptElement.MatchString(line) {
			out = append(out, Violation{Path: file, Rule: RuleScriptElement, Detail: `El("script", ...) builds a script element`, Line: i + 1})
		}
	}

	// Rules 3a and 3c: string literals.
	tokenFile := fset.AddFile(file+"#tokens", -1, len(data))
	var s scanner.Scanner
	s.Init(tokenFile, data, nil, 0)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok != token.STRING {
			continue
		}
		text := lit
		if unquoted, err := strconv.Unquote(lit); err == nil {
			text = unquoted
		}
		out = append(out, scanText(file, text, tokenFile.Line(pos), true)...)
	}
	return out, nil
}

// ParseAllowlist reads tab-separated rows: path, rule, reason, removed-by.
// Blank lines and lines starting with # are skipped.
func ParseAllowlist(r io.Reader) ([]Entry, error) {
	var entries []Entry
	scan := bufio.NewScanner(r)
	for n := 1; scan.Scan(); n++ {
		line := strings.TrimRight(scan.Text(), "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) != 4 {
			return nil, fmt.Errorf("allowlist line %d: want 4 tab-separated columns (path, rule, reason, removed-by), got %d", n, len(cols))
		}
		for i := range cols {
			cols[i] = strings.TrimSpace(cols[i])
			if cols[i] == "" {
				return nil, fmt.Errorf("allowlist line %d: column %d is empty", n, i+1)
			}
		}
		entries = append(entries, Entry{Path: cols[0], Rule: cols[1], Reason: cols[2], RemovedBy: cols[3]})
	}
	return entries, scan.Err()
}

// LoadAllowlist reads the allowlist at file. A missing file is an empty list.
func LoadAllowlist(file string) ([]Entry, error) {
	handle, err := os.Open(file)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	return ParseAllowlist(handle)
}

// Apply removes allowlisted violations and reports allowlist entries that match
// no violation, so the list shrinks as the exceptions go away.
func Apply(violations []Violation, allow []Entry) (remaining []Violation, stale []Entry) {
	used := make([]bool, len(allow))
	for _, v := range violations {
		cleared := false
		for i, entry := range allow {
			if entry.Path == v.Path && entry.Rule == v.Rule {
				used[i] = true
				cleared = true
			}
		}
		if !cleared {
			remaining = append(remaining, v)
		}
	}
	for i, entry := range allow {
		if !used[i] {
			stale = append(stale, entry)
		}
	}
	return remaining, stale
}
