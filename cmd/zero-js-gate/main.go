// Command zero-js-gate enforces the zero-authored-JavaScript rule for this
// repository: application behavior is Go and GoSX, with the exceptions named in
// zero-js-allowlist.txt. See the README for the rules.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fatal(err)
	}
	files, err := ListFiles(root)
	if err != nil {
		fatal(err)
	}
	violations, err := Scan(root, files)
	if err != nil {
		fatal(err)
	}
	allow, err := LoadAllowlist(filepath.Join(root, "zero-js-allowlist.txt"))
	if err != nil {
		fatal(err)
	}
	remaining, stale := Apply(violations, allow)
	for _, v := range remaining {
		if v.Line > 0 {
			fmt.Printf("%s:%d: %s: %s\n", v.Path, v.Line, v.Rule, v.Detail)
		} else {
			fmt.Printf("%s: %s: %s\n", v.Path, v.Rule, v.Detail)
		}
	}
	for _, entry := range stale {
		fmt.Printf("%s: %s: %s %s (remove it from zero-js-allowlist.txt)\n", entry.Path, RuleStaleAllowlist, entry.Path, entry.Rule)
	}
	if len(remaining) > 0 || len(stale) > 0 {
		fmt.Fprintln(os.Stderr, "zero-js gate: failed")
		os.Exit(1)
	}
	fmt.Printf("zero-js gate: %d files scanned, %d allowlisted exceptions\n", len(files), len(allow))
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
	fmt.Fprintln(os.Stderr, "zero-js gate:", err)
	os.Exit(1)
}
