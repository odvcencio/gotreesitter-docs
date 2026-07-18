// Command build-authoring-wasm produces the browser-only grammar-authoring
// engine consumed by the GoSX authoring surface (app/authoring). Unlike
// build-playground-wasm, it ships no pre-generated grammar blobs: grammargen
// compiles the user's grammar.json live, in the browser, so the only outputs
// are wasm binaries (plus, for the worker, a small generated JS bootstrap —
// see workerBootstrapJS below for why that's generated here rather than
// hand-authored and checked in).
//
// Phase 1 builds two wasm binaries:
//   - authoring.wasm: the GoSX island (cmd/authoring-wasm) that owns the DOM
//     — unchanged in shape from Phase 0.
//   - authoring-worker.wasm: a plain (non-GoSX) js/wasm build
//     (cmd/authoring-worker-wasm) that runs the actual grammargen
//     compile+parse+diagnose+highlight loop inside a Web Worker, off the
//     main thread, so a heavy grammar cannot freeze the tab. It is loaded by
//     authoring-worker.js, a small generated bootstrap (importScripts the
//     already-served /gosx/standard-go-wasm_exec.js Go/wasm shim, then
//     fetch+instantiate+run authoring-worker.wasm) — mechanical Go/wasm
//     boilerplate, not application logic, so it is generated at build time
//     into public/authoring/ and gitignored, exactly like the wasm binaries
//     themselves. This keeps the repository's "no hand-authored JS" rule
//     (see scripts/verify-production.sh) intact: every byte of behavior is
//     still authored in Go.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/odvcencio/gotreesitter/grammargen"
)

// baseGrammars is the Phase 2 inheritance base catalog: every entry becomes
// a public/authoring/bases/<name>.grammar.json asset the browser can fetch
// and use as an "extends" base (see internal/authoringengine.MergeGrammarJSON
// and cmd/authoring-wasm's base picker). grammargen exposes ~20 built-in
// `func XGrammar() *Grammar` constructors (see grep 'func \w+Grammar\(\)
// \*Grammar' in the grammargen package); this list is a deliberately curated
// subset, not all of them — measured natively (go1.26, this machine) via
// GenerateLanguage+GenerateWithReport (CompileWithContext's actual double
// generation pass):
//
//	calc ~1ms, json ~7ms, ini ~7ms, mustache ~10ms  (instant — good demo defaults)
//	lox ~3.3s combined                              (Phase 1's existing "heavy" example)
//	go ~4.9s combined                                (headline real-language base)
//	typescript ~15s combined, javascript ~25s combined (heavy, but selectable)
//
// swift (generate alone: 53s), kotlin (>67s and still running), fortran
// (2m34s combined!), and markdown (16.5s combined, plus its own known
// heredoc-shaped generation caveats) were measured and excluded: they blow
// far past any budget a live in-browser Web Worker compile can reasonably
// offer (cmd/authoring-wasm's hardBudget is 15s), so shipping them as
// authoring bases would just be a guaranteed-broken "pick this and it never
// finishes" trap. They still export cleanly as *data* (ExportGrammarJSON
// does not fail for any of them) — only live in-browser compilation is
// impractical. Re-evaluate if grammargen's generation performance improves.
var baseGrammars = []struct {
	Name  string
	Build func() *grammargen.Grammar
}{
	{"calc", grammargen.CalcGrammar},
	{"json", grammargen.JSONGrammar},
	{"ini", grammargen.INIGrammar},
	{"mustache", grammargen.MustacheGrammar},
	{"lox", grammargen.LoxGrammar},
	{"go", grammargen.GoGrammar},
	{"typescript", grammargen.TypescriptGrammar},
	{"javascript", grammargen.JavascriptGrammar},
}

type baseAsset struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	RuleCount int    `json:"ruleCount"`
}

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fatal(err)
	}
	outDir := filepath.Join(root, "public", "authoring")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}

	assets, err := writeBaseAssets(outDir)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %d base grammar.json assets to %s\n", len(assets), filepath.Join(outDir, "bases"))

	buildWASM(root, filepath.Join(outDir, "authoring.wasm"), "./cmd/authoring-wasm")
	buildWASM(root, filepath.Join(outDir, "authoring-worker.wasm"), "./cmd/authoring-worker-wasm")
	writeWorkerBootstrap(filepath.Join(outDir, "authoring-worker.js"), filepath.Join(outDir, "authoring-worker.wasm"))
}

// writeBaseAssets exports every entry in baseGrammars to
// <outDir>/bases/<name>.grammar.json via grammargen.ExportGrammarJSON — the
// same pure, side-effect-free exporter internal/authoringengine.MergeGrammarJSON
// re-imports at merge time — plus a bases/index.json manifest the browser's
// base picker fetches to populate its <select>.
func writeBaseAssets(outDir string) ([]baseAsset, error) {
	dir := filepath.Join(outDir, "bases")
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	assets := make([]baseAsset, 0, len(baseGrammars))
	for _, entry := range baseGrammars {
		g := entry.Build()
		if g == nil {
			return nil, fmt.Errorf("base %q: Build returned a nil grammar", entry.Name)
		}
		data, err := grammargen.ExportGrammarJSON(g)
		if err != nil {
			return nil, fmt.Errorf("base %q: ExportGrammarJSON: %w", entry.Name, err)
		}
		filename := entry.Name + ".grammar.json"
		if err := os.WriteFile(filepath.Join(dir, filename), data, 0o644); err != nil {
			return nil, fmt.Errorf("base %q: write asset: %w", entry.Name, err)
		}
		assets = append(assets, baseAsset{
			Name:      entry.Name,
			URL:       "/authoring/bases/" + filename,
			RuleCount: len(g.RuleOrder),
		})
		fmt.Printf("  base %-12s %5d rules  %8d bytes\n", entry.Name, len(g.RuleOrder), len(data))
	}

	index, err := json.MarshalIndent(assets, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), index, 0o644); err != nil {
		return nil, err
	}
	return assets, nil
}

func buildWASM(root, output, pkg string) {
	command := exec.Command(
		"go", "build",
		"-trimpath",
		"-ldflags=-s -w",
		"-o", output,
		pkg,
	)
	command.Dir = root
	command.Env = append(os.Environ(), "GOWORK=off", "GOOS=js", "GOARCH=wasm")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		fatal(err)
	}
	info, err := os.Stat(output)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("built %s (%d bytes)\n", output, info.Size())
}

// writeWorkerBootstrap generates the Worker's entry script. A Worker always
// needs a JS entry point (there is no way to start one with pure wasm — the
// Worker constructor requires a script URL, and Go's wasm output needs the
// host bindings wasm_exec.js provides), so this is unavoidable, but it is
// intentionally minimal: importScripts the standard Go wasm_exec.js the
// framework already serves for every "go-wasm" runtime engine on this page
// (/gosx/standard-go-wasm_exec.js — see main_stub reference in
// cmd/authoring-wasm), then fetch+instantiate+run authoring-worker.wasm
// (resolved relative to this script's own URL so it survives any future
// path/versioning changes). All actual behavior — the compile loop, the
// message protocol, diagnostics, highlighting — lives in
// cmd/authoring-worker-wasm's Go source, not here.
func writeWorkerBootstrap(scriptPath, wasmPath string) {
	sum, err := fileSHA256Prefix(wasmPath)
	if err != nil {
		fatal(err)
	}
	script := "// GENERATED by cmd/build-authoring-wasm — do not hand-edit.\n" +
		"// wasm=" + sum + "\n" +
		workerBootstrapJS
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		fatal(err)
	}
	info, err := os.Stat(scriptPath)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("built %s (%d bytes)\n", scriptPath, info.Size())
}

const workerBootstrapJS = `(function () {
  "use strict";
  try {
    self.importScripts("/gosx/standard-go-wasm_exec.js");
  } catch (err) {
    self.postMessage({ seq: -1, generateError: "authoring worker: failed to load the Go wasm runtime: " + err });
    return;
  }
  var GoCtor = self.__gosx_standard_go_wasm_ctor || self.Go;
  if (typeof GoCtor !== "function") {
    self.postMessage({ seq: -1, generateError: "authoring worker: standard Go wasm runtime unavailable" });
    return;
  }
  var go = new GoCtor();
  var wasmURL = new URL("authoring-worker.wasm", self.location.href).href;
  fetch(wasmURL)
    .then(function (resp) {
      if (!resp.ok) {
        throw new Error("fetch " + wasmURL + ": " + resp.status);
      }
      return resp.arrayBuffer();
    })
    .then(function (bytes) {
      return WebAssembly.instantiate(bytes, go.importObject);
    })
    .then(function (result) {
      go.run(result.instance);
    })
    .catch(function (err) {
      self.postMessage({ seq: -1, generateError: "authoring worker failed to start: " + err });
    });
})();
`

func fileSHA256Prefix(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:6]), nil
}

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
			return "", fmt.Errorf("cannot find repository go.mod")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "build-authoring-wasm:", err)
	os.Exit(1)
}
