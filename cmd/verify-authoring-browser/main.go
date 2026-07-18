// Command verify-authoring-browser proves the Phase 0 grammar-authoring
// loop actually executes in a browser: it loads the running /authoring
// page, waits for the seed calc grammar.json to compile and parse, and
// asserts the rendered syntax tree is non-empty.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	base := strings.TrimRight(os.Getenv("AUTHORING_BASE_URL"), "/")
	if base == "" {
		base = "http://127.0.0.1:18081"
	}
	chrome, err := chromePath()
	if err != nil {
		fatal(err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chrome),
		chromedp.Headless,
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.WindowSize(1280, 900),
	)
	allocator, cancelAllocator := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAllocator()
	ctx, cancel := chromedp.NewContext(allocator)
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, 60*time.Second)
	defer cancelTimeout()

	if err := chromedp.Run(ctx,
		chromedp.Navigate(base+"/authoring"),
		chromedp.WaitVisible("#ag-grammar", chromedp.ByQuery),
	); err != nil {
		fatal(err)
	}
	if err := waitForText(ctx, "#ag-status", "Compiled"); err != nil {
		fatal(err)
	}

	var treeText string
	if err := chromedp.Run(ctx, chromedp.Text("#ag-tree", &treeText, chromedp.ByQuery)); err != nil {
		fatal(err)
	}
	treeText = strings.TrimSpace(treeText)
	if treeText == "" || strings.Contains(treeText, "No tree rows") || strings.Contains(treeText, "Loading the browser grammar compiler") {
		fatal(fmt.Errorf("#ag-tree is empty after compiling the seed grammar: %q", treeText))
	}
	if !strings.Contains(treeText, "expression") || !strings.Contains(treeText, "number") {
		fatal(fmt.Errorf("#ag-tree does not look like a calc parse tree: %q", treeText))
	}

	var statusText string
	if err := chromedp.Run(ctx, chromedp.Text("#ag-status", &statusText, chromedp.ByQuery)); err != nil {
		fatal(err)
	}

	fmt.Println("authoring loop proved: seed grammar.json compiled and parsed in-browser")
	fmt.Println("status:", statusText)
	fmt.Println("tree (first 400 chars):", truncate(treeText, 400))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func waitForText(ctx context.Context, selector, want string) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var value string
		if err := chromedp.Run(ctx, chromedp.Text(selector, &value, chromedp.ByQuery)); err == nil && strings.Contains(value, want) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("%s never contained %q", selector, want)
}

func chromePath() (string, error) {
	if configured := os.Getenv("AUTHORING_CHROME"); configured != "" {
		return configured, nil
	}
	for _, candidate := range []string{"google-chrome", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("Chrome executable not found")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "verify-authoring-browser:", err)
	os.Exit(1)
}
