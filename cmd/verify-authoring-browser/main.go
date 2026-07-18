// Command verify-authoring-browser proves the Phase 1 grammar-authoring
// loop actually executes in a browser, off the main thread:
//
//  1. the seed calc grammar.json compiles and parses through the background
//     Web Worker (not a main-thread fallback) — asserted via the
//     data-compile-via attribute cmd/authoring-wasm's render() sets;
//  2. editing the grammar to a deliberately ambiguous one produces a
//     non-empty LR-conflicts panel;
//  3. the highlight preview pane is populated for the seed grammar's sample.
//
// It also exercises a heavy grammar (grammargen.LoxGrammar(), the design
// doc's canonical "expensive" example) end to end through the worker and
// reports the observed wall-clock time, and probes that the main thread
// keeps answering trivial JS evaluations while that heavy compile is in
// flight — the actual point of moving the compiler into a Worker.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/odvcencio/gotreesitter/grammargen"
)

// ambiguousGrammarJSON has no precedence/associativity annotations at all —
// "expr -> expr '+' expr | NUMBER" — so grammargen's LR builder cannot avoid
// at least one shift/reduce conflict. Verified natively
// (internal/authoringengine) to report exactly 1 conflict.
const ambiguousGrammarJSON = `{
  "name": "ambiguous",
  "rules": {
    "expr": {
      "type": "CHOICE",
      "members": [
        {
          "type": "SEQ",
          "members": [
            {"type":"SYMBOL","name":"expr"},
            {"type":"STRING","value":"+"},
            {"type":"SYMBOL","name":"expr"}
          ]
        },
        {"type":"PATTERN","value":"[0-9]+"}
      ]
    }
  },
  "extras": [{"type":"PATTERN","value":"\\s"}],
  "conflicts": [],
  "externals": [],
  "inline": [],
  "supertypes": []
}`

const loxSample = `class Greeter {
  init(name) {
    this.name = name;
  }
  greet() {
    print "Hello, " + this.name + "!";
  }
}
var g = Greeter("Lox");
g.greet();
for (var i = 0; i < 3; i = i + 1) {
  print i;
}
`

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
	ctx, cancelTimeout := context.WithTimeout(ctx, 120*time.Second)
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

	// (1) The seed grammar compiled+parsed through the Worker, not the
	// main-thread fallback.
	via, err := attribute(ctx, "#ag-root", "data-compile-via")
	if err != nil {
		fatal(err)
	}
	if via != "worker" {
		fatal(fmt.Errorf("seed grammar compiled via %q, want %q (background Worker) — main-thread fallback engaged", via, "worker"))
	}
	fmt.Println("seed calc grammar compiled+parsed through the background Web Worker")

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
	fmt.Println("worker response rendered a real syntax tree")

	// The seed calc grammar itself has 40 precedence-resolved conflicts
	// (verified natively in internal/authoringengine) — confirm the panel
	// reflects that rather than staying empty/placeholder.
	var seedConflictSummary string
	if err := chromedp.Run(ctx, chromedp.Text("#ag-conflict-summary", &seedConflictSummary, chromedp.ByQuery)); err != nil {
		fatal(err)
	}
	if !strings.Contains(seedConflictSummary, "conflict") {
		fatal(fmt.Errorf("#ag-conflict-summary does not mention conflicts for the seed grammar: %q", seedConflictSummary))
	}
	fmt.Println("conflicts panel populated for the seed grammar:", strings.TrimSpace(seedConflictSummary))

	// (3) Highlight preview is populated for the seed sample (operators in
	// "1 + 2 * (3 - 4)").
	var highlightText string
	if err := chromedp.Run(ctx, chromedp.Text("#ag-highlight", &highlightText, chromedp.ByQuery)); err != nil {
		fatal(err)
	}
	highlightText = strings.TrimSpace(highlightText)
	if highlightText == "" || strings.Contains(highlightText, "No highlight spans") {
		fatal(fmt.Errorf("#ag-highlight is empty for the seed sample: %q", highlightText))
	}
	var highlightSpanCount int
	if err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(
		`document.querySelectorAll('#ag-highlight .ag-hl').length`, &highlightSpanCount,
	)); err != nil {
		fatal(err)
	}
	if highlightSpanCount == 0 {
		fatal(fmt.Errorf("#ag-highlight has no styled .ag-hl spans"))
	}
	fmt.Printf("highlight preview populated: %d styled span(s), text %q\n", highlightSpanCount, truncate(highlightText, 120))

	// (2) A deliberately ambiguous grammar produces a non-empty conflicts
	// panel.
	if err := setValueAndDispatchInput(ctx, "#ag-grammar", ambiguousGrammarJSON); err != nil {
		fatal(err)
	}
	if err := setValueAndDispatchInput(ctx, "#ag-source", "1 + 2 + 3"); err != nil {
		fatal(err)
	}
	if err := waitForText(ctx, "#ag-status", "ambiguous"); err != nil {
		fatal(err)
	}
	var conflictCount int
	if err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(
		`document.querySelectorAll('#ag-conflicts .ag-conflict').length`, &conflictCount,
	)); err != nil {
		fatal(err)
	}
	if conflictCount == 0 {
		fatal(fmt.Errorf("deliberately ambiguous grammar produced an empty #ag-conflicts panel"))
	}
	var conflictSummary string
	if err := chromedp.Run(ctx, chromedp.Text("#ag-conflict-summary", &conflictSummary, chromedp.ByQuery)); err != nil {
		fatal(err)
	}
	fmt.Printf("deliberately-conflicting grammar rendered %d conflict row(s): %s\n", conflictCount, strings.TrimSpace(conflictSummary))

	viaAfterEdit, err := attribute(ctx, "#ag-root", "data-compile-via")
	if err != nil {
		fatal(err)
	}
	if viaAfterEdit != "worker" {
		fatal(fmt.Errorf("ambiguous-grammar compile went via %q, want %q", viaAfterEdit, "worker"))
	}

	// Heavier-grammar timing + main-thread responsiveness, through the same
	// Worker.
	runHeavyGrammarCheck(ctx, base)

	fmt.Println("authoring Phase 1 verified: worker-hosted compile, conflict diagnostics, and highlight preview all proved in-browser")
}

// runHeavyGrammarCheck feeds grammargen.LoxGrammar() (the design doc's
// canonical "expensive" grammar — natively ~1.7s combined for the two LR
// generation passes authoringengine.CompileWithContext runs) through the
// editor, times how long the worker takes to answer, and — the actual point
// of Phase 1 — proves the main thread kept responding to trivial JS calls
// throughout instead of freezing.
func runHeavyGrammarCheck(ctx context.Context, base string) {
	g := grammargen.LoxGrammar()
	data, err := grammargen.ExportGrammarJSON(g)
	if err != nil {
		fatal(fmt.Errorf("export lox grammar.json: %w", err))
	}
	fmt.Printf("submitting a heavy grammar (lox, %d bytes of grammar.json) to the worker…\n", len(data))

	if err := setValueAndDispatchInput(ctx, "#ag-source", loxSample); err != nil {
		fatal(err)
	}
	started := time.Now()
	if err := setValueAndDispatchInput(ctx, "#ag-grammar", string(data)); err != nil {
		fatal(err)
	}

	// Probe main-thread responsiveness while the worker is (presumably)
	// still compiling: repeatedly round-trip a trivial Runtime.Evaluate call
	// through the SAME renderer process. If the page's main thread were
	// blocked by the compile (i.e. Phase 0's inline architecture), these
	// calls would queue up behind it; since the compile now runs in a
	// Worker, they should stay fast throughout.
	deadline := started.Add(30 * time.Second)
	var maxProbeLatency time.Duration
	probes := 0
	status := ""
	for time.Now().Before(deadline) {
		probeStart := time.Now()
		var v int
		if err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(`21 * 2`, &v)); err != nil {
			fatal(fmt.Errorf("main-thread responsiveness probe failed: %w", err))
		}
		latency := time.Since(probeStart)
		if latency > maxProbeLatency {
			maxProbeLatency = latency
		}
		probes++
		if v != 42 {
			fatal(fmt.Errorf("main-thread probe returned %d, want 42 (evaluate context corrupted?)", v))
		}
		if err := chromedp.Run(ctx, chromedp.Text("#ag-status", &status, chromedp.ByQuery)); err != nil {
			fatal(err)
		}
		if strings.Contains(status, "Compiled \"lox\"") {
			break
		}
		time.Sleep(80 * time.Millisecond)
	}
	elapsed := time.Since(started)
	if !strings.Contains(status, "Compiled \"lox\"") {
		fatal(fmt.Errorf("lox grammar did not finish compiling within 30s (last status: %q)", status))
	}

	fmt.Printf("lox grammar compiled+parsed via the worker in %s wall-clock (status: %q)\n", elapsed.Round(time.Millisecond), strings.TrimSpace(status))
	fmt.Printf("main-thread responsiveness while the worker compiled lox: %d probe(s), worst Runtime.Evaluate round trip %s\n", probes, maxProbeLatency.Round(time.Millisecond))
	// A generous ceiling: chromedp's Runtime.Evaluate round trip is itself
	// network+IPC bound (not just page compute), so this is deliberately
	// loose — the point is proving it stays in the tens/low-hundreds of
	// milliseconds throughout a multi-second background compile, not
	// spiking to seconds the way a blocked main thread would.
	if maxProbeLatency > 2*time.Second {
		fatal(fmt.Errorf("main-thread probe latency spiked to %s while the worker compiled lox — main thread may have been blocked", maxProbeLatency))
	}

	viaLox, err := attribute(ctx, "#ag-root", "data-compile-via")
	if err != nil {
		fatal(err)
	}
	if viaLox != "worker" {
		fatal(fmt.Errorf("lox compile went via %q, want %q", viaLox, "worker"))
	}
	_ = base
}

func setValueAndDispatchInput(ctx context.Context, selector, value string) error {
	return chromedp.Run(ctx,
		chromedp.SetValue(selector, value, chromedp.ByQuery),
		chromedp.EvaluateAsDevTools(
			fmt.Sprintf(`document.querySelector(%q).dispatchEvent(new Event('input', {bubbles:true}))`, selector),
			nil,
		),
	)
}

func attribute(ctx context.Context, selector, name string) (string, error) {
	attrs := map[string]string{}
	if err := chromedp.Run(ctx, chromedp.Attributes(selector, &attrs, chromedp.ByQuery)); err != nil {
		return "", err
	}
	return attrs[name], nil
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
