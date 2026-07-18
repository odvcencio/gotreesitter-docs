// Command build-authoring-wasm produces the browser-only grammar-authoring
// engine consumed by the GoSX authoring surface (app/authoring). Unlike
// build-playground-wasm, it ships no pre-generated grammar blobs: grammargen
// compiles the user's grammar.json live, in the browser, so the only output
// is the wasm binary itself.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fatal(err)
	}
	output := filepath.Join(root, "public", "authoring", "authoring.wasm")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		fatal(err)
	}
	command := exec.Command(
		"go", "build",
		"-trimpath",
		"-ldflags=-s -w",
		"-o", output,
		"./cmd/authoring-wasm",
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
