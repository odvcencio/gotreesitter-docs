package docs

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type docExample struct {
	page      string
	index     int
	info      string
	code      string
	output    string
	program   bool
	hasOutput bool
}

type exampleImport struct {
	spec string
	ref  string
}

var (
	fencedBlock         = regexp.MustCompile("(?ms)^```([^\\s`]+)[^\\n]*\\r?\\n(.*?)^```\\s*$")
	leadingImportBlock  = regexp.MustCompile(`(?s)^\s*import\s*\((.*?)\)\s*`)
	leadingSingleImport = regexp.MustCompile(`(?s)^\s*import\s+([^\n]+)\s*\n`)
)

func TestGoDocumentationExamplesCompile(t *testing.T) {
	goMod, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	goSum, err := os.ReadFile("../go.sum")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	// Use the local import path from the authoring example while retaining all
	// dependencies from this repository's copied module files.
	goMod = []byte(strings.Replace(string(goMod),
		"module github.com/odvcencio/gotreesitter-docs",
		"module example.com", 1))
	if err := os.WriteFile(filepath.Join(root, "go.mod"), goMod, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.sum"), goSum, 0o600); err != nil {
		t.Fatal(err)
	}

	examples := readDocumentationExamples(t)
	for _, example := range examples {
		if err := writeExample(root, example); err != nil {
			t.Fatalf("write %s block %d: %v", example.page, example.index, err)
		}
	}
	writeExampleSupport(t, root, examples)

	cmd := exec.Command("go", "test", "-p=2", "./examples/...")
	cmd.Dir = root
	cmd.Env = offlineGoEnv()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile documentation examples: %v\n%s", err, output)
	}

	counts := make(map[string][3]int)
	for _, example := range examples {
		c := counts[example.page]
		c[0]++
		c[1]++ // All blocks are compiled in the batched go test above.
		if example.hasOutput {
			binaryDir := filepath.Join(root, "examples", exampleDir(example))
			cmd := exec.Command("go", "run", ".")
			cmd.Dir = binaryDir
			cmd.Env = offlineGoEnv()
			got, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Fatalf("run %s block %d: %v\n%s", example.page, example.index, err, exitErr.Stderr)
				}
				t.Fatalf("run %s block %d: %v", example.page, example.index, err)
			}
			if string(got) != example.output {
				t.Errorf("%s block %d output mismatch\n got: %q\nwant: %q", example.page, example.index, got, example.output)
			}
			c[2]++
		}
		counts[example.page] = c
	}
	for _, page := range sortedCountPages(counts) {
		c := counts[page]
		t.Logf("examples %s: blocks=%d compiled=%d run=%d", page, c[0], c[1], c[2])
	}
}

func readDocumentationExamples(t *testing.T) []docExample {
	t.Helper()
	paths, err := filepath.Glob("../content/docs/*.md")
	if err != nil {
		t.Fatal(err)
	}
	var out []docExample
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		page := filepath.Base(path)
		blocks := fencedBlock.FindAllStringSubmatchIndex(string(body), -1)
		var parsed []struct {
			lang, info, code string
			start, end       int
		}
		for _, block := range blocks {
			info := string(body[block[2]:block[3]])
			lang := strings.Fields(info)[0]
			content := string(body[block[4]:block[5]])
			parsed = append(parsed, struct {
				lang, info, code string
				start, end       int
			}{lang, info, content, block[0], block[1]})
		}
		for i, block := range parsed {
			if block.lang != "go" {
				continue
			}
			example := docExample{page: page, index: 1, info: block.info, code: block.code}
			// Keep numbering local to the page for useful diagnostics.
			for j := range out {
				if out[j].page == page && out[j].index >= example.index {
					example.index = out[j].index + 1
				}
			}
			example.program = startsWithPackage(example.code)
			if example.program && hasTextOutputAfter(string(body), block.end, parsed, i) {
				for j := i + 1; j < len(parsed); j++ {
					if parsed[j].lang == "text" && isAdjacentOutputLabel(string(body[block.end:parsed[j].start])) {
						example.output = parsed[j].code
						example.hasOutput = true
						break
					}
					if parsed[j].lang != "text" {
						break
					}
				}
			}
			out = append(out, example)
		}
	}
	if heroSnippet != "" {
		example := docExample{page: "home", index: 1, info: "go hero", code: heroSnippet}
		out = append(out, example)
	}
	return out
}

func startsWithPackage(code string) bool {
	for _, line := range strings.Split(code, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		return strings.HasPrefix(line, "package ")
	}
	return false
}

func hasTextOutputAfter(body string, end int, blocks []struct {
	lang, info, code string
	start, end       int
}, index int) bool {
	if index+1 >= len(blocks) || blocks[index+1].lang != "text" {
		return false
	}
	return isAdjacentOutputLabel(body[end:blocks[index+1].start])
}

func isAdjacentOutputLabel(gap string) bool {
	gap = strings.TrimSpace(gap)
	return gap == "" || gap == "Running it prints:" || gap == "Output:"
}

func writeExample(root string, example docExample) error {
	dir := filepath.Join(root, "examples", exampleDir(example))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	code := strings.TrimSpace(example.code)
	if !example.program {
		code = buildFragmentSource(example, code)
	}
	code = "// source: " + example.page + " block " + strconv.Itoa(example.index) + "\n" + code + "\n"
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte(code), 0o600); err != nil {
		return err
	}
	if example.page == "authoring-languages.md" && example.index == 4 {
		if err := os.WriteFile(filepath.Join(dir, "kvconf.bin"), nil, 0o600); err != nil {
			return err
		}
	}
	if example.page == "authoring-languages.md" && example.index == 7 {
		for name, value := range map[string][]byte{
			"pawn.bin":               nil,
			"queries/highlights.scm": nil,
		} {
			path := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(path, value, 0o600); err != nil {
				return err
			}
		}
	}
	return nil
}

func exampleDir(example docExample) string {
	return strings.TrimSuffix(example.page, ".md") + "/block-" + fmt.Sprintf("%02d", example.index)
}

func buildFragmentSource(example docExample, code string) string {
	if example.page == "authoring-languages.md" && example.index == 4 {
		marker := "//go:embed kvconf.bin\nvar kvconfBlob []byte\n"
		if strings.HasPrefix(code, marker) {
			return "package main\n\n" + renderImports(importsFor(example.page)) + marker +
				"\nfunc main() {\n" + localPrelude(example) + strings.TrimSpace(strings.TrimPrefix(code, marker)) + "\n}\n"
		}
	}
	var imported string
	if m := leadingImportBlock.FindStringSubmatchIndex(code); m != nil {
		imported = code[m[2]:m[3]]
		code = code[m[1]:]
	} else if m := leadingSingleImport.FindStringSubmatchIndex(code); m != nil {
		imported = code[m[2]:m[3]]
		code = code[m[1]:]
	}
	imports := importsFor(example.page)
	for _, spec := range strings.Split(imported, "\n") {
		spec = strings.TrimSpace(spec)
		if spec == "" || strings.HasPrefix(spec, "//") {
			continue
		}
		imports = appendUniqueImport(imports, exampleImport{spec: spec})
	}
	importsText := renderImports(imports)
	aliases := topLevelPrelude(example)
	if isTopLevelFragment(code) {
		return "package main\n\n" + importsText + aliases + "\n" + strings.TrimSpace(code)
	}
	return "package main\n\n" + importsText + aliases + "\nfunc main() {\n" +
		localPrelude(example) + strings.TrimSpace(code) + "\n}\n"
}

func isTopLevelFragment(code string) bool {
	_, err := parser.ParseFile(token.NewFileSet(), "example.go", "package main\n"+code, parser.AllErrors)
	return err == nil
}

func importsFor(page string) []exampleImport {
	base := []exampleImport{
		{spec: `"fmt"`, ref: "fmt.Sprintf"},
		{spec: `gts "github.com/odvcencio/gotreesitter"`, ref: "gts.NewParser"},
		{spec: `"github.com/odvcencio/gotreesitter/grammars"`, ref: "grammars.GoLanguage"},
	}
	switch page {
	case "home":
		base[1] = exampleImport{spec: `gotreesitter "github.com/odvcencio/gotreesitter"`, ref: "gotreesitter.NewParser"}
	case "authoring-languages.md":
		base = append(base,
			exampleImport{spec: `_ "embed"`},
			exampleImport{spec: `"log"`, ref: "log.Fatal"},
			exampleImport{spec: `"os"`, ref: "os.ReadFile"},
			exampleImport{spec: `"github.com/odvcencio/gotreesitter/grammargen"`, ref: "grammargen.NewGrammar"},
			exampleImport{spec: `"github.com/odvcencio/gotreesitter/taproot"`, ref: "taproot.Parse"},
			exampleImport{spec: `"github.com/odvcencio/gotreesitter/taproot/walk"`, ref: "walk.ParseFromBlob"},
			exampleImport{spec: `"example.com/kvconf"`, ref: "kvconf.Grammar"},
		)
	case "code-navigation.md", "incremental-parsing.md":
		base = append(base, exampleImport{spec: `"log"`, ref: "log.Fatal"})
	case "parsers-in-depth.md":
		base = append(base,
			exampleImport{spec: `"context"`, ref: "context.Background"},
			exampleImport{spec: `"errors"`, ref: "errors.Is"},
			exampleImport{spec: `"log"`, ref: "log.Fatal"},
			exampleImport{spec: `"sync/atomic"`, ref: "atomic.StoreUint32"},
		)
	case "syntax-highlighting.md":
		base = append(base, exampleImport{spec: `"log"`, ref: "log.Fatal"})
	case "queries.md":
		base = append(base, exampleImport{spec: `"log"`, ref: "log.Fatal"})
	case "languages.md":
		base = append(base, exampleImport{spec: `_ "embed"`, ref: ""})
	}
	return base
}

func appendUniqueImport(imports []exampleImport, next exampleImport) []exampleImport {
	for _, item := range imports {
		if item.spec == next.spec {
			return imports
		}
	}
	return append(imports, next)
}

func renderImports(imports []exampleImport) string {
	var b strings.Builder
	b.WriteString("import (\n")
	refs := make([]string, 0, len(imports))
	for _, item := range imports {
		b.WriteString("\t" + item.spec + "\n")
		if item.ref != "" {
			refs = append(refs, item.ref)
		}
	}
	b.WriteString(")\n\nvar (\n")
	for i, ref := range refs {
		b.WriteString(fmt.Sprintf("\t_exampleImport%d = %s\n", i, ref))
	}
	b.WriteString(")\n\n")
	return b.String()
}

func topLevelPrelude(example docExample) string {
	switch example.page {
	case "external-scanners.md":
		if example.index == 1 {
			return "type ExternalLexer = gts.ExternalLexer\n\n"
		}
		if example.index == 3 {
			return "func NewPawnScanner(*gts.Language) gts.ExternalScanner { return nil }\n\n"
		}
	case "incremental-parsing.md":
		if example.index == 2 {
			return "type Point = gts.Point\n\n"
		}
	case "code-navigation.md":
		if example.index == 2 {
			return "type Range = gts.Range\ntype Point = gts.Point\n\n"
		}
	case "language-injection.md":
		if example.index == 2 {
			return "type Tree = gts.Tree\ntype Range = gts.Range\ntype Node = gts.Node\n\n"
		}
	}
	return ""
}

func localPrelude(example docExample) string {
	switch example.page {
	case "home":
		return "src := []byte(\"package main\\nfunc main() {}\\n\")\n"
	case "architecture.md":
		return "src := []byte(\"package main\\nfunc main() {}\\n\")\nvar edit gts.InputEdit\nvar newSrc []byte\n"
	case "authoring-languages.md":
		switch example.index {
		case 4:
			return "src := []byte(\"name = \\\"fable\\\"\\n\")\n"
		case 5, 6:
			return "var kvconfBlob []byte\nsrc := []byte(\"name = \\\"fable\\\"\\n\")\n"
		}
	case "code-navigation.md":
		if example.index == 3 {
			return parserTreePrelude()
		}
	case "external-scanners.md":
		if example.index == 3 {
			return "var pawnBlob []byte\n"
		}
	case "getting-started.md":
		switch example.index {
		case 2:
			return parseRootPrelude()
		case 3:
			return parseRootPrelude() + "fn := root.NamedChild(1)\n"
		case 4:
			return "src := []byte(\"package main\\nfunc main() {}\\n\")\n"
		case 5:
			return "lang := grammars.GoLanguage()\nparser := gts.NewParser(lang)\n_ = lang\n"
		}
	case "incremental-parsing.md":
		if example.index == 1 {
			return "lang := grammars.GoLanguage()\nparser := gts.NewParser(lang)\nsrc := []byte(\"package main\\nfunc main() {}\\n\")\nnewSrc := []byte(\"package main\\nfunc main() { println(1) }\\n\")\n"
		}
	case "language-injection.md":
		switch example.index {
		case 1:
			return "src := []byte(\"# doc\\n\\n```go\\nfunc main() {}\\n```\\n\")\n"
		case 3:
			return "fenceStart := uint32(0)\nfenceEnd := uint32(0)\nfullDocument := []byte(\"func main() {}\")\n"
		}
	case "languages-introspection.md":
		if example.index == 2 || example.index == 3 {
			return "lang := grammars.GoLanguage()\n"
		}
	case "languages.md":
		return ""
	case "parsers-in-depth.md":
		switch example.index {
		case 1, 5:
			return "lang := grammars.GoLanguage()\nsrc := []byte(\"package main\\nfunc main() {}\\n\")\n"
		case 2:
			return "lang := grammars.GoLanguage()\nparser := gts.NewParser(lang)\nsrc := []byte(\"package main\\nfunc main() {}\\n\")\nctx := context.Background()\n_ = lang\n"
		case 3:
			return "parser := gts.NewParser(grammars.GoLanguage())\nsrc := []byte(\"package main\\nfunc main() {}\\n\")\n"
		case 4:
			return "src := []byte(\"package main\\nfunc main() {}\\n\")\nparser := gts.NewParser(grammars.GoLanguage())\nvar oldTree *gts.Tree\n"
		}
	case "queries.md":
		if example.index == 1 {
			return "src := []byte(\"package main\\nfunc main() {}\\n\")\n"
		}
		if example.index >= 2 && example.index <= 7 {
			return "lang := grammars.GoLanguage()\n"
		}
		if example.index == 8 {
			return "src := []byte(\"package main\\nfunc main() {}\\n\")\nlang := grammars.GoLanguage()\nparser := gts.NewParser(lang)\ntree, _ := parser.Parse(src)\nq, _ := gts.NewQuery(\"(source_file) @root\", lang)\n_ = src\n_ = parser\n"
		}
	case "recovery-and-correctness.md":
		return ""
	case "syntax-highlighting.md":
		switch example.index {
		case 3:
			return "src := []byte(\"package main\\nfunc main() {}\\n\")\nentry := grammars.DetectLanguage(\"main.go\")\nlang := entry.Language()\nhl, _ := gts.NewHighlighter(lang, entry.HighlightQuery)\nvar oldTree *gts.Tree\nnewSrc := src\nvar ranges []gts.HighlightRange\n_ = entry\n_ = lang\n"
		case 4:
			return "entry := grammars.DetectLanguage(\"main.go\")\nlang := entry.Language()\n"
		}
	case "syntax-trees-and-nodes.md":
		switch example.index {
		case 1:
			return "src := []byte(\"package main\\n\\nfunc main() {}\\n\")\n"
		case 2:
			return parseRootPrelude() + "node := root.NamedChild(0)\n"
		case 3, 5, 8:
			return parseRootPrelude()
		case 6:
			return parseRootPrelude() + "fn := root.NamedChild(1)\n"
		case 7:
			return parseRootPrelude() + "fn := root.NamedChild(1)\n"
		}
	case "tree-cursors.md":
		switch example.index {
		case 2:
			return parseRootPrelude()
		case 3, 4, 5:
			return parserTreePrelude()
		}
	}
	return ""
}

func parseRootPrelude() string {
	return "src := []byte(\"package main\\n\\nfunc main() {}\\n\")\nlang := grammars.GoLanguage()\nparser := gts.NewParser(lang)\ntree, _ := parser.Parse(src)\nroot := tree.RootNode()\n_ = src\n_ = lang\n_ = parser\n_ = tree\n_ = root\n"
}

func parserTreePrelude() string {
	return "src := []byte(\"package main\\n\\nfunc main() {}\\n\")\nlang := grammars.GoLanguage()\nparser := gts.NewParser(lang)\ntree, _ := parser.Parse(src)\n_ = src\n_ = lang\n_ = parser\n_ = tree\n"
}

func offlineGoEnv() []string {
	env := os.Environ()
	for _, pair := range []string{"GOWORK=off", "GOPROXY=off", "GOFLAGS=-mod=mod"} {
		env = append(env, pair)
	}
	return env
}

func writeExampleSupport(t *testing.T, root string, examples []docExample) {
	t.Helper()
	var kvconf, pawnScanner string
	for _, example := range examples {
		if example.page == "authoring-languages.md" && example.index == 2 {
			kvconf = example.code
		}
		if example.page == "external-scanners.md" && example.index == 2 {
			pawnScanner = example.code
		}
	}
	if kvconf == "" {
		return
	}
	dir := filepath.Join(root, "kvconf")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kvconf.go"), []byte(kvconf), 0o600); err != nil {
		t.Fatal(err)
	}
	if pawnScanner != "" {
		pawnDir := filepath.Join(root, "examples", "authoring-languages", "block-07")
		if err := os.WriteFile(filepath.Join(pawnDir, "scanner.go"), []byte(pawnScanner), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func sortedCountPages(counts map[string][3]int) []string {
	pages := make([]string, 0, len(counts))
	for page := range counts {
		pages = append(pages, page)
	}
	sort.Strings(pages)
	return pages
}
