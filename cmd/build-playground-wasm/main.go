// Command build-playground-wasm produces the browser-only gotreesitter engine
// consumed by the GoSX playground surface.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/odvcencio/gotreesitter/grammars"
)

const expectedGrammarCount = 206

type grammarAsset struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Bytes int    `json:"bytes"`
}

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fatal(err)
	}
	output := filepath.Join(root, "public", "playground", "runtime.wasm")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		fatal(err)
	}
	assets, err := writeGrammarAssets(root)
	if err != nil {
		fatal(err)
	}
	command := exec.Command(
		"go", "build",
		"-tags", "grammar_blobs_external",
		"-trimpath",
		"-ldflags=-s -w",
		"-o", output,
		"./cmd/playground-wasm",
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
	fmt.Printf("built %s (%d bytes) with %d lazy grammar blobs\n", output, info.Size(), len(assets))
}

func writeGrammarAssets(root string) ([]grammarAsset, error) {
	entries := grammars.AllLanguages()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	if len(entries) != expectedGrammarCount {
		return nil, fmt.Errorf("grammar registry contains %d languages, want %d", len(entries), expectedGrammarCount)
	}

	dir := filepath.Join(root, "public", "playground", "grammars")
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	assets := make([]grammarAsset, 0, len(entries))
	for _, entry := range entries {
		blob := grammars.BlobByName(entry.Name)
		if len(blob) == 0 {
			return nil, fmt.Errorf("grammar %q has no distributable blob", entry.Name)
		}
		sum := sha256.Sum256(blob)
		filename := entry.Name + "." + hex.EncodeToString(sum[:6]) + ".bin"
		if err := os.WriteFile(filepath.Join(dir, filename), blob, 0o644); err != nil {
			return nil, err
		}
		assets = append(assets, grammarAsset{
			Name:  entry.Name,
			URL:   "/playground/grammars/" + filename,
			Bytes: len(blob),
		})
	}
	index, err := json.Marshal(assets)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), index, 0o644); err != nil {
		return nil, err
	}
	return assets, nil
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
	fmt.Fprintln(os.Stderr, "build-playground-wasm:", err)
	os.Exit(1)
}
