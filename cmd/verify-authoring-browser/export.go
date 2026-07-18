// Phase 3 (export) verification. See main.go's package doc, items 8-9.
//
// "Validated" per format, and exactly what that means:
//
//   - grammar.json: the downloaded bytes parse as JSON and contain the
//     delta-added "power" rule. This is a real structural/content check
//     (json.Unmarshal + a map lookup), not a substring/pattern match.
//   - .go: the downloaded bytes are written into a fresh, empty scratch Go
//     module that requires exactly the github.com/odvcencio/gotreesitter
//     version this repo's own go.mod pins, and `go build ./...` is run
//     against it — offline (GOPROXY=off, GOSUMDB=off), using only the
//     already-resolved local module cache. This is the STRONGER check the
//     task calls for: it proves the export is not just syntactically
//     plausible Go but genuinely compiles against the real dependency,
//     exactly as an author who downloaded it would experience.
//   - parser.c: the downloaded bytes are checked for the standard
//     tree-sitter parser.c structural markers, AND (when gcc is on PATH,
//     which it is in this environment) run through a real
//     `gcc -fsyntax-only` against a vendored, unmodified copy of
//     tree-sitter's own tree_sitter/parser.h (testdata/tree_sitter/parser.h)
//     plus one small, explicitly-documented compatibility shim
//     (testdata/tree_sitter/compat_shim.h) that bridges a real naming gap
//     between what gotreesitter's grammargen.EmitC emits and this specific
//     (current, real) header revision — see that shim file's own doc
//     comment for the full explanation, including the one benign,
//     pre-existing warning class (not an error) it does NOT paper over. If
//     gcc were unavailable this would fall back to structural-markers-only
//     and say so explicitly; that fallback path is not needed in this
//     environment.
//
// Every download in this file is a REAL browser download: chromedp
// configures Chrome's actual download behavior via
// browser.SetDownloadBehavior and this program waits for a genuine
// browser.EventDownloadProgress "completed" CDP event before reading
// whatever file Chrome itself wrote to disk. Nothing here intercepts or
// simulates the click/Blob/anchor-download mechanism cmd/authoring-wasm's
// triggerDownload actually uses in production.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
)

//go:embed testdata/tree_sitter
var cHeaderFixture embed.FS

// downloadTracker observes real Chrome download lifecycle events (via CDP,
// configured through browser.SetDownloadBehavior) so this program can wait
// for a specific button click's download to actually complete, rather than
// guessing at a fixed sleep.
type downloadTracker struct {
	dir string

	mu        sync.Mutex
	began     map[string]string // guid -> SuggestedFilename
	completed map[string]bool
	filePaths map[string]string // guid -> FilePath, when Chrome reports one
}

// newDownloadTracker configures Chrome (via CDP) to allow downloads into dir
// without a save-as prompt, naming each saved file after its GUID (matching
// chromedp's own TestDownloadIntoDir pattern), and registers listeners for
// the two download lifecycle events this program correlates.
func newDownloadTracker(ctx context.Context, dir string) *downloadTracker {
	tr := &downloadTracker{
		dir:       dir,
		began:     map[string]string{},
		completed: map[string]bool{},
		filePaths: map[string]string{},
	}
	chromedp.ListenTarget(ctx, func(v any) {
		switch ev := v.(type) {
		case *browser.EventDownloadWillBegin:
			tr.mu.Lock()
			tr.began[ev.GUID] = ev.SuggestedFilename
			tr.mu.Unlock()
		case *browser.EventDownloadProgress:
			if ev.State == browser.DownloadProgressStateCompleted {
				tr.mu.Lock()
				tr.completed[ev.GUID] = true
				if ev.FilePath != "" {
					tr.filePaths[ev.GUID] = ev.FilePath
				}
				tr.mu.Unlock()
			}
		}
	})
	if err := chromedp.Run(ctx,
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath(dir).
			WithEventsEnabled(true),
	); err != nil {
		fatal(fmt.Errorf("configure browser download behavior: %w", err))
	}
	return tr
}

// snapshotSeen returns the set of download GUIDs already completed at this
// moment, so a caller can later ask "wait for a NEW one" without racing
// downloads that finished before it started watching.
func (tr *downloadTracker) snapshotSeen() map[string]bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	seen := make(map[string]bool, len(tr.completed))
	for g := range tr.completed {
		seen[g] = true
	}
	return seen
}

// waitForNewDownload blocks (polling) until a download GUID absent from seen
// reaches the "completed" state, then returns its guid, suggested filename,
// and the real path Chrome wrote it to on disk.
func (tr *downloadTracker) waitForNewDownload(seen map[string]bool, timeout time.Duration) (guid, filename, path string, err error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		tr.mu.Lock()
		for g := range tr.completed {
			if !seen[g] {
				fn := tr.began[g]
				fp := tr.filePaths[g]
				tr.mu.Unlock()
				if fp == "" {
					fp = filepath.Join(tr.dir, g)
				}
				return g, fn, fp, nil
			}
		}
		tr.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	return "", "", "", fmt.Errorf("no new download completed within %s", timeout)
}

// clickExportAndWait clicks one export button and waits for the resulting
// real browser download to complete, failing loudly (including the current
// #ag-export-status text, for diagnosis) if it doesn't within timeout.
//
// This dispatches the click via a JS .click() call (chromedp.EvaluateAsDevTools),
// matching the existing setValueAndDispatchInput/selectAndDispatchChange
// helpers elsewhere in this package, rather than chromedp.Click's
// coordinate-based Input.dispatchMouseEvent simulation — the latter, verified
// empirically against this exact page, unreliably misses the export toolbar
// (positioned below the two-column editor grid) in headless Chrome at this
// window size, silently producing no click at all rather than an error.
func clickExportAndWait(ctx context.Context, downloads *downloadTracker, seen map[string]bool, selector, format string, timeout time.Duration) (guid, filename, path string) {
	if err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(
		fmt.Sprintf(`document.querySelector(%q).click()`, selector), nil,
	)); err != nil {
		fatal(fmt.Errorf("click %s: %w", selector, err))
	}
	guid, filename, path, err := downloads.waitForNewDownload(seen, timeout)
	if err != nil {
		status, _ := textContent(ctx, "#ag-export-status")
		fatal(fmt.Errorf("export %s (%s): %w (last #ag-export-status: %q)", format, selector, err, strings.TrimSpace(status)))
	}
	return guid, filename, path
}

// checkExportButtons is design item (8): against the default seed page
// state (calc base + power delta, already proven compiled through the
// worker by the caller before this runs), click each of the three export
// buttons and validate the real downloaded content — see this file's
// package doc for exactly what "validated" means per format.
func checkExportButtons(ctx context.Context, downloads *downloadTracker) {
	seen := downloads.snapshotSeen()

	for _, tc := range []struct {
		selector string
		format   string
	}{
		{"#ag-export-json", "json"},
		{"#ag-export-go", "go"},
		{"#ag-export-c", "c"},
	} {
		guid, filename, path := clickExportAndWait(ctx, downloads, seen, tc.selector, tc.format, 60*time.Second)
		seen[guid] = true

		data, err := os.ReadFile(path)
		if err != nil {
			fatal(fmt.Errorf("export %s: read downloaded file %s: %w", tc.format, path, err))
		}
		if len(data) == 0 {
			fatal(fmt.Errorf("export %s: downloaded file %s is empty", tc.format, path))
		}
		fmt.Printf("export %s: real browser download %q (%d bytes)\n", tc.format, filename, len(data))

		switch tc.format {
		case "json":
			validateExportedGrammarJSON(filename, data)
		case "go":
			validateExportedGoSource(filename, data)
		case "c":
			validateExportedParserC(filename, data)
		}
	}
	fmt.Println("all three export formats downloaded and validated for the default seed (calc base + power delta)")
}

func validateExportedGrammarJSON(filename string, data []byte) {
	if !strings.HasSuffix(filename, ".grammar.json") {
		fatal(fmt.Errorf("grammar.json export filename %q does not end in %q", filename, ".grammar.json"))
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		fatal(fmt.Errorf("exported grammar.json does not parse as JSON: %w", err))
	}
	rules, ok := payload["rules"].(map[string]any)
	if !ok {
		fatal(fmt.Errorf("exported grammar.json has no \"rules\" object"))
	}
	if _, ok := rules["power"]; !ok {
		fatal(fmt.Errorf("exported grammar.json is missing the delta-added %q rule", "power"))
	}
	if _, ok := rules["expression"]; !ok {
		fatal(fmt.Errorf("exported grammar.json is missing the base %q rule", "expression"))
	}
	fmt.Println("  validated: parses as JSON; \"rules\" contains both the delta-added \"power\" rule and the base \"expression\" rule")
}

func validateExportedGoSource(filename string, data []byte) {
	if !strings.HasSuffix(filename, ".go") {
		fatal(fmt.Errorf("go export filename %q does not end in .go", filename))
	}
	content := string(data)
	for _, marker := range []string{"package ", "import . \"github.com/odvcencio/gotreesitter/grammargen\"", "func Authored"} {
		if !strings.Contains(content, marker) {
			fatal(fmt.Errorf("exported .go is missing expected marker %q:\n%s", marker, content))
		}
	}
	fmt.Println("  structural markers present: package clause, grammargen dot import, an AuthoredXGrammar generator func")

	dir, err := os.MkdirTemp("", "authoring-export-go-build-")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0o644); err != nil {
		fatal(err)
	}

	gtsVersion := gotreesitterModuleVersion()
	runGo := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off", "GOSUMDB=off")
		out, err := cmd.CombinedOutput()
		if err != nil {
			fatal(fmt.Errorf("%v failed: %w\n%s", args, err, out))
		}
	}
	runGo("go", "mod", "init", "authoringexportverify")
	runGo("go", "mod", "edit", "-require=github.com/odvcencio/gotreesitter@"+gtsVersion)
	runGo("go", "build", "./...")
	fmt.Printf("  validated: a real `go build` of a fresh scratch module (offline, local module cache only) against github.com/odvcencio/gotreesitter %s succeeded\n", gtsVersion)
}

func validateExportedParserC(filename string, data []byte) {
	if filename != "parser.c" {
		fatal(fmt.Errorf("c export filename: got %q, want %q", filename, "parser.c"))
	}
	content := string(data)
	for _, marker := range []string{"#include <tree_sitter/parser.h>", "#define LANGUAGE_VERSION", "tree_sitter_calc("} {
		if !strings.Contains(content, marker) {
			fatal(fmt.Errorf("exported parser.c is missing expected marker %q", marker))
		}
	}
	fmt.Println("  structural markers present: #include <tree_sitter/parser.h>, #define LANGUAGE_VERSION, tree_sitter_calc(")

	if !gccAvailable() {
		fmt.Println("  gcc not found on PATH — validated structural markers ONLY for this format, not a real compile/syntax check")
		return
	}

	dir, err := os.MkdirTemp("", "authoring-export-c-check-")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "parser.c"), data, 0o644); err != nil {
		fatal(err)
	}
	if err := extractCHeaderFixture(dir); err != nil {
		fatal(fmt.Errorf("extract vendored tree_sitter/parser.h fixture: %w", err))
	}

	cmd := exec.Command("gcc", "-fsyntax-only", "-I.", "parser.c")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fatal(fmt.Errorf("gcc -fsyntax-only parser.c failed:\n%s", out))
	}
	if trimmed := strings.TrimSpace(string(out)); trimmed != "" {
		fmt.Printf("  gcc -fsyntax-only succeeded (exit 0) with warnings — see testdata/tree_sitter/compat_shim.h for the one known, pre-existing, non-fatal warning class this is expected to produce for ABI<15 grammars like calc:\n%s\n", trimmed)
	} else {
		fmt.Println("  gcc -fsyntax-only succeeded (exit 0), zero warnings")
	}
	fmt.Println("  validated: a real `gcc -fsyntax-only` against a vendored, unmodified tree_sitter/parser.h (+ one documented compatibility shim) succeeded")
}

func gccAvailable() bool {
	_, err := exec.LookPath("gcc")
	return err == nil
}

// extractCHeaderFixture writes the embedded testdata/tree_sitter/*.h fixture
// files into dir/tree_sitter/, so a parser.c written alongside dir can
// `#include <tree_sitter/parser.h>` via a plain `-I.` include path.
func extractCHeaderFixture(dir string) error {
	return fs.WalkDir(cHeaderFixture, "testdata/tree_sitter", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("testdata", path)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := cHeaderFixture.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// gotreesitterModuleVersion reads the exact github.com/odvcencio/gotreesitter
// version this repo's own go.mod pins (via `go list -m`, run from the
// repository root) so validateExportedGoSource's scratch build module always
// matches — no hand-maintained version string to drift out of sync.
func gotreesitterModuleVersion() string {
	root, err := repositoryRoot()
	if err != nil {
		fatal(err)
	}
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}", "github.com/odvcencio/gotreesitter")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		fatal(fmt.Errorf("go list -m github.com/odvcencio/gotreesitter: %w\n%s", err, out))
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		fatal(fmt.Errorf("go list -m github.com/odvcencio/gotreesitter returned an empty version"))
	}
	return version
}

// repositoryRoot walks up from the current working directory to find the
// nearest go.mod — mirrors cmd/build-authoring-wasm's helper of the same
// name/shape.
func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if info, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("cannot find repository go.mod above %s", dir)
		}
		dir = parent
	}
}

// checkGoBaseExportReusesCache is design item (9): after the caller has
// already selected the "go" base and confirmed it compiled through the
// worker (checkGoBase), export it and confirm the export completes fast —
// i.e. the worker reused its cached compile (see
// cmd/authoring-worker-wasm's package doc) instead of re-running LR-table
// generation. reuseUpperBound is generous on purpose: it only needs to rule
// out a full regeneration (which cmd/build-authoring-wasm's baseGrammars doc
// measures natively at several seconds for the go base, and slower still
// under Go/wasm), not pin a tight latency budget that could flake on a
// loaded CI machine.
func checkGoBaseExportReusesCache(ctx context.Context, downloads *downloadTracker) {
	seen := downloads.snapshotSeen()

	started := time.Now()
	guid, filename, path := clickExportAndWait(ctx, downloads, seen, "#ag-export-json", "json", 15*time.Second)
	elapsed := time.Since(started)
	seen[guid] = true

	data, err := os.ReadFile(path)
	if err != nil {
		fatal(err)
	}
	if len(data) == 0 {
		fatal(fmt.Errorf("go base grammar.json export is empty"))
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		fatal(fmt.Errorf("go base exported grammar.json does not parse: %w", err))
	}
	if name, _ := payload["name"].(string); name != "go" {
		fatal(fmt.Errorf("go base exported grammar.json \"name\": got %q, want %q", name, "go"))
	}

	fmt.Printf("go base export (grammar.json, %q, %d bytes) completed in %s\n", filename, len(data), elapsed.Round(time.Millisecond))
	const reuseUpperBound = 5 * time.Second
	if elapsed > reuseUpperBound {
		fatal(fmt.Errorf("go base export took %s, want under %s — the worker may have re-run LR-table generation instead of reusing its cache (see cmd/authoring-worker-wasm's package doc)", elapsed, reuseUpperBound))
	}
	fmt.Printf("go base export completed in %s (< %s) — the worker's cached compile was reused rather than re-running LR-table generation for this heavy grammar\n", elapsed.Round(time.Millisecond), reuseUpperBound)
}
