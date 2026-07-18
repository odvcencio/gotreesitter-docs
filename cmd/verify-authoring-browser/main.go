// Command verify-authoring-browser proves the grammar-authoring loop
// actually executes in a browser, off the main thread, across both
// Phase 1 (single grammar) and Phase 2 (base + delta inheritance) behavior:
//
//  1. the seed delta (calc base + a "power" rule) compiles and parses
//     through the background Web Worker (not a main-thread fallback) —
//     asserted via the data-compile-via attribute cmd/authoring-wasm's
//     render() sets;
//  2. the base picker (#ag-base) is populated from
//     public/authoring/bases/index.json and defaults to calc;
//  3. the parsed sample's tree contains BOTH a base rule ("expression",
//     "number" — calc's own) AND the delta-added rule ("power") — proving
//     the base+delta merge actually took effect, not just that something
//     rendered;
//  4. switching to a different base (json) with a different delta (adding a
//     "nan_literal" rule) re-proves the same thing end to end through an
//     explicit user action, not just the page's own seed state;
//  5. LR conflicts and the highlight preview still populate for the
//     extended (base+delta) grammar;
//  6. the "blank / full grammar" option reproduces Phase 1 exactly: editing
//     to a deliberately ambiguous full grammar.json produces a non-empty
//     conflicts panel, and a heavy grammar (grammargen.LoxGrammar()) compiles
//     end to end through the worker without freezing the main thread;
//  7. selecting GoGrammar as the base — the star "inherit a real language"
//     demo — compiles through the worker without freezing the main thread.
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
// (internal/authoringengine) to report exactly 1 conflict. It is a
// standalone full grammar.json (its own "name"), used with the base picker
// set to "" (blank / full grammar) — Phase 1's exact original behavior.
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

// jsonDeltaJSON is the explicit Phase 2 "switch bases" proof: a delta
// against the json base that overrides json's hidden "_value" choice to add
// one more alternative — a brand-new "nan_literal" rule (JSON has no NaN
// literal; this extends it to accept one, JSON5-style). Verified natively
// (see the base+delta merge check below) to parse `[1, NaN, "x"]` into a
// tree containing json's own "array"/"number"/"string" rules AND the
// delta-added "nan_literal" rule.
const jsonDeltaJSON = `{
  "rules": {
    "_value": {
      "type": "CHOICE",
      "members": [
        {"name": "object", "type": "SYMBOL"},
        {"name": "array", "type": "SYMBOL"},
        {"name": "number", "type": "SYMBOL"},
        {"name": "string", "type": "SYMBOL"},
        {"name": "true", "type": "SYMBOL"},
        {"name": "false", "type": "SYMBOL"},
        {"name": "null", "type": "SYMBOL"},
        {"name": "nan_literal", "type": "SYMBOL"}
      ]
    },
    "nan_literal": {"type": "STRING", "value": "NaN"}
  }
}`

const jsonDeltaSample = `[1, NaN, "x"]`

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

const goSample = `package main

func add(a int, b int) int {
	return a + b
}

func main() {
	x := add(1, 2)
	_ = x
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
	ctx, cancelTimeout := context.WithTimeout(ctx, 240*time.Second)
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

	// The seed delta (calc base + "power" rule) compiled+parsed through the
	// Worker, not the main-thread fallback.
	via, err := attribute(ctx, "#ag-root", "data-compile-via")
	if err != nil {
		fatal(err)
	}
	if via != "worker" {
		fatal(fmt.Errorf("seed delta compiled via %q, want %q (background Worker) — main-thread fallback engaged", via, "worker"))
	}
	fmt.Println("seed delta (calc base + power rule) compiled+parsed through the background Web Worker")

	// (2) The base picker populated from bases/index.json and defaulted to calc.
	checkBasePickerPopulated(ctx)

	// (3) The default page state already proves base+delta merge: the tree
	// contains both a base rule and the delta-added rule.
	checkTreeContainsTypes(ctx, "default seed (calc base + power delta)", []string{"expression", "number", "power"})

	// (5) Conflicts + highlight populate for the extended (base+delta) grammar.
	checkConflictsAndHighlightPopulated(ctx, "calc+power")

	// (4) An explicit base switch (json + a different delta) re-proves the
	// merge end to end through a real user action.
	checkExplicitBaseSwitch(ctx)

	// (6) The "blank / full grammar" option reproduces Phase 1 exactly.
	checkBlankModeAmbiguousGrammar(ctx)
	runHeavyGrammarCheck(ctx, base)

	// (7) GoGrammar as a base — the star "inherit a real language" demo —
	// compiles through the worker without freezing the main thread.
	checkGoBase(ctx)

	fmt.Println("authoring Phase 2 verified: base picker, base+delta merge (twice, on two different bases), blank/full-grammar mode, conflict diagnostics, highlight preview, and a real-language (go) base all proved in-browser through the worker")
}

// checkBasePickerPopulated asserts design item (a): the base picker is
// populated from public/authoring/bases/index.json (the blank option plus
// every shipped base) and defaults to calc.
func checkBasePickerPopulated(ctx context.Context) {
	var optionCount int
	if err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(
		`document.querySelectorAll('#ag-base option').length`, &optionCount,
	)); err != nil {
		fatal(err)
	}
	// blank + at least calc/json/ini/mustache/lox/go/typescript/javascript.
	const wantMinOptions = 9
	if optionCount < wantMinOptions {
		fatal(fmt.Errorf("#ag-base has %d <option>s, want at least %d (blank + shipped bases)", optionCount, wantMinOptions))
	}
	baseCount, err := attribute(ctx, "#ag-root", "data-base-count")
	if err != nil {
		fatal(err)
	}
	if baseCount == "" {
		fatal(fmt.Errorf("#ag-root data-base-count was never set — bases/index.json fetch did not complete"))
	}
	selected, err := elementValue(ctx, "#ag-base")
	if err != nil {
		fatal(err)
	}
	if selected != "calc" {
		fatal(fmt.Errorf("#ag-base default selection: got %q, want %q", selected, "calc"))
	}
	baseInfo, err := textContent(ctx, "#ag-base-info")
	if err != nil {
		fatal(err)
	}
	if !strings.Contains(baseInfo, "calc") || !strings.Contains(baseInfo, "rule") {
		fatal(fmt.Errorf("#ag-base-info does not describe the calc base: %q", baseInfo))
	}
	fmt.Printf("base picker populated from bases/index.json: %d <option>s (data-base-count=%s), defaulted to %q — %q\n", optionCount, baseCount, selected, strings.TrimSpace(baseInfo))
}

// checkTreeContainsTypes asserts #ag-tree's rendered node types include every
// entry in want — the "prove the merge actually took effect" check design
// item (b) calls for: both a base rule and the delta-added rule must appear
// in the SAME parsed tree, not merely "something rendered".
func checkTreeContainsTypes(ctx context.Context, label string, want []string) {
	var treeText string
	if err := chromedp.Run(ctx, chromedp.Text("#ag-tree", &treeText, chromedp.ByQuery)); err != nil {
		fatal(err)
	}
	treeText = strings.TrimSpace(treeText)
	if treeText == "" || strings.Contains(treeText, "No tree rows") || strings.Contains(treeText, "Loading the browser grammar compiler") {
		fatal(fmt.Errorf("%s: #ag-tree is empty: %q", label, treeText))
	}
	var missing []string
	for _, want := range want {
		if !strings.Contains(treeText, want) {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		fatal(fmt.Errorf("%s: #ag-tree is missing node type(s) %v (proves the merge did NOT take effect): tree=%q", label, missing, truncate(treeText, 500)))
	}
	fmt.Printf("%s: tree contains every expected node type %v (merge proven — base rule(s) and the delta-added rule both reached the parsed tree)\n", label, want)
}

// checkConflictsAndHighlightPopulated asserts design item (c): LR conflicts
// and the highlight preview still populate for the currently-loaded extended
// (base+delta) grammar.
func checkConflictsAndHighlightPopulated(ctx context.Context, label string) {
	var conflictSummary string
	if err := chromedp.Run(ctx, chromedp.Text("#ag-conflict-summary", &conflictSummary, chromedp.ByQuery)); err != nil {
		fatal(err)
	}
	if !strings.Contains(conflictSummary, "conflict") {
		fatal(fmt.Errorf("%s: #ag-conflict-summary does not mention conflicts: %q", label, conflictSummary))
	}
	fmt.Printf("%s: conflicts panel populated: %s\n", label, strings.TrimSpace(conflictSummary))

	var highlightText string
	if err := chromedp.Run(ctx, chromedp.Text("#ag-highlight", &highlightText, chromedp.ByQuery)); err != nil {
		fatal(err)
	}
	highlightText = strings.TrimSpace(highlightText)
	if highlightText == "" || strings.Contains(highlightText, "No highlight spans") {
		fatal(fmt.Errorf("%s: #ag-highlight is empty", label))
	}
	var highlightSpanCount int
	if err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(
		`document.querySelectorAll('#ag-highlight .ag-hl').length`, &highlightSpanCount,
	)); err != nil {
		fatal(err)
	}
	if highlightSpanCount == 0 {
		fatal(fmt.Errorf("%s: #ag-highlight has no styled .ag-hl spans", label))
	}
	fmt.Printf("%s: highlight preview populated: %d styled span(s), text %q\n", label, highlightSpanCount, truncate(highlightText, 120))
}

// checkExplicitBaseSwitch is design item (b)'s second, more deliberate
// proof: an explicit user action (change the base picker, retype the delta
// and sample) — not just the page's own seed state — re-proves the merge.
//
// The delta and sample are set BEFORE the base switch, deliberately: once
// json's grammar.json finishes fetching, onBaseChange's loadBaseAndRun calls
// run() automatically using whatever is currently in #ag-grammar/#ag-source
// — setting them first means that very first post-fetch compile already
// uses the intended nan_literal delta, instead of racing a second, later
// debounced run() against a transient compile of the *previous* (calc)
// delta merged onto json.
func checkExplicitBaseSwitch(ctx context.Context) {
	if err := setValueAndDispatchInput(ctx, "#ag-grammar", jsonDeltaJSON); err != nil {
		fatal(err)
	}
	if err := setValueAndDispatchInput(ctx, "#ag-source", jsonDeltaSample); err != nil {
		fatal(err)
	}
	if err := selectAndDispatchChange(ctx, "#ag-base", "json"); err != nil {
		fatal(err)
	}
	if err := waitForText(ctx, "#ag-base-info", "json"); err != nil {
		fatal(fmt.Errorf("base switch to json: %w", err))
	}
	if err := waitForText(ctx, "#ag-status", `Compiled "json"`); err != nil {
		fatal(fmt.Errorf("base switch to json: %w", err))
	}
	via, err := attribute(ctx, "#ag-root", "data-compile-via")
	if err != nil {
		fatal(err)
	}
	if via != "worker" {
		fatal(fmt.Errorf("json base+delta compile went via %q, want %q", via, "worker"))
	}
	checkTreeContainsTypes(ctx, "explicit base switch (json base + nan_literal delta)", []string{"array", "number", "string", "nan_literal"})

	// chromedp.Text reads innerText, which reflects the panel header's CSS
	// text-transform:uppercase (see .panelhd in docs.css) — compare
	// case-insensitively; the DOM's actual textContent (what
	// cmd/authoring-wasm's updateEditorLabel sets) is lowercase.
	label, err := textContent(ctx, "#ag-editor-label")
	if err != nil {
		fatal(err)
	}
	lowerLabel := strings.ToLower(label)
	if !strings.Contains(lowerLabel, "delta") || !strings.Contains(lowerLabel, "json") {
		fatal(fmt.Errorf("#ag-editor-label was not relabeled for the json base: %q", label))
	}
	fmt.Printf("#ag-editor-label relabeled for the active base: %q\n", strings.TrimSpace(label))
}

// checkBlankModeAmbiguousGrammar asserts the "blank / full grammar" option
// reproduces Phase 1 exactly: #ag-grammar is the whole grammar (its own
// "name", no base merge), and a deliberately ambiguous one produces a
// non-empty conflicts panel.
func checkBlankModeAmbiguousGrammar(ctx context.Context) {
	if err := selectAndDispatchChange(ctx, "#ag-base", ""); err != nil {
		fatal(err)
	}
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
		fatal(fmt.Errorf("blank mode: deliberately ambiguous grammar produced an empty #ag-conflicts panel"))
	}
	var conflictSummary string
	if err := chromedp.Run(ctx, chromedp.Text("#ag-conflict-summary", &conflictSummary, chromedp.ByQuery)); err != nil {
		fatal(err)
	}
	fmt.Printf("blank/full-grammar mode: deliberately-conflicting grammar rendered %d conflict row(s): %s\n", conflictCount, strings.TrimSpace(conflictSummary))

	viaAfterEdit, err := attribute(ctx, "#ag-root", "data-compile-via")
	if err != nil {
		fatal(err)
	}
	if viaAfterEdit != "worker" {
		fatal(fmt.Errorf("ambiguous-grammar compile went via %q, want %q", viaAfterEdit, "worker"))
	}
}

// runHeavyGrammarCheck feeds grammargen.LoxGrammar() (the design doc's
// canonical "expensive" grammar) through the editor in blank/full-grammar
// mode, times how long the worker takes to answer, and proves the main
// thread kept responding to trivial JS calls throughout instead of freezing.
// Assumes #ag-base is already "" (checkBlankModeAmbiguousGrammar leaves it
// there).
func runHeavyGrammarCheck(ctx context.Context, base string) {
	g := grammargen.LoxGrammar()
	data, err := grammargen.ExportGrammarJSON(g)
	if err != nil {
		fatal(fmt.Errorf("export lox grammar.json: %w", err))
	}
	fmt.Printf("submitting a heavy grammar (lox, %d bytes of grammar.json, blank/full-grammar mode) to the worker…\n", len(data))

	if err := setValueAndDispatchInput(ctx, "#ag-source", loxSample); err != nil {
		fatal(err)
	}
	started := time.Now()
	if err := setValueAndDispatchInput(ctx, "#ag-grammar", string(data)); err != nil {
		fatal(err)
	}

	status, maxProbeLatency, probes := probeMainThreadUntil(ctx, started, 30*time.Second, `Compiled "lox"`)
	elapsed := time.Since(started)
	if !strings.Contains(status, `Compiled "lox"`) {
		fatal(fmt.Errorf("lox grammar did not finish compiling within 30s (last status: %q)", status))
	}

	fmt.Printf("lox grammar compiled+parsed via the worker in %s wall-clock (status: %q)\n", elapsed.Round(time.Millisecond), strings.TrimSpace(status))
	fmt.Printf("main-thread responsiveness while the worker compiled lox: %d probe(s), worst Runtime.Evaluate round trip %s\n", probes, maxProbeLatency.Round(time.Millisecond))
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

// checkGoBase is design item (d): select grammargen.GoGrammar() (the
// headline real-language base) as #ag-base, clear the delta so the grammar
// generated is Go unmodified, and verify it compiles+parses through the
// worker without freezing the main thread. Go is measured natively (this
// repo's own instrumentation, go1.26) at ~4.9s combined for the two LR
// generation passes authoringengine.CompileWithContext runs (generate +
// diagnose) — noticeably heavier than lox's ~3.3s — so this uses a longer
// deadline than the lox check and tolerates the main-thread watchdog
// restarting the worker at most once (a Go base sitting right at the edge
// of cmd/authoring-wasm's hardBudget is expected, not a bug in this check).
func checkGoBase(ctx context.Context) {
	// Set the delta/sample BEFORE switching bases — same race-avoidance
	// reasoning as checkExplicitBaseSwitch: once go's grammar.json finishes
	// fetching, the auto-triggered run() reads whatever is currently in the
	// editors, so setting them first guarantees the one (heavy) compile this
	// check measures is already the intended one, not a discarded compile of
	// go merged with whatever the previous check (blank mode + lox) left
	// behind, doubling the actual worker time this check would observe.
	if err := setValueAndDispatchInput(ctx, "#ag-grammar", "{}"); err != nil {
		fatal(err)
	}
	if err := setValueAndDispatchInput(ctx, "#ag-source", goSample); err != nil {
		fatal(err)
	}

	started := time.Now()
	if err := selectAndDispatchChange(ctx, "#ag-base", "go"); err != nil {
		fatal(err)
	}
	if err := waitForText(ctx, "#ag-base-info", "go"); err != nil {
		fatal(fmt.Errorf("base switch to go: %w", err))
	}
	baseInfo, err := textContent(ctx, "#ag-base-info")
	if err != nil {
		fatal(err)
	}
	fmt.Printf("selected go as base: %s\n", strings.TrimSpace(baseInfo))

	status, maxProbeLatency, probes := probeMainThreadUntil(ctx, started, 90*time.Second, `Compiled "go"`)
	elapsed := time.Since(started)
	if !strings.Contains(status, `Compiled "go"`) {
		fatal(fmt.Errorf("go base did not finish compiling within 90s (last status: %q) — see the design doc's risk #1 (generation latency) for why a real-language base can be this heavy", status))
	}
	fmt.Printf("go base compiled+parsed via the worker in %s wall-clock (status: %q)\n", elapsed.Round(time.Millisecond), strings.TrimSpace(status))
	fmt.Printf("main-thread responsiveness while the worker compiled go: %d probe(s), worst Runtime.Evaluate round trip %s\n", probes, maxProbeLatency.Round(time.Millisecond))
	if maxProbeLatency > 2*time.Second {
		fatal(fmt.Errorf("main-thread probe latency spiked to %s while the worker compiled go — main thread may have been blocked", maxProbeLatency))
	}

	viaGo, err := attribute(ctx, "#ag-root", "data-compile-via")
	if err != nil {
		fatal(err)
	}
	if viaGo != "worker" {
		fatal(fmt.Errorf("go base compile went via %q, want %q", viaGo, "worker"))
	}

	var treeText string
	if err := chromedp.Run(ctx, chromedp.Text("#ag-tree", &treeText, chromedp.ByQuery)); err != nil {
		fatal(err)
	}
	fmt.Printf("go base tree preview: %s\n", truncate(strings.TrimSpace(treeText), 300))
}

// probeMainThreadUntil repeatedly round-trips a trivial Runtime.Evaluate
// call through the SAME renderer process while #ag-status is polled for
// wantSubstr, up to deadline after started. If the page's main thread were
// blocked by the compile, these calls would queue up behind it; since the
// compile runs in a Worker, they should stay fast throughout. Returns the
// last observed status text, the worst probe latency, and how many probes ran.
func probeMainThreadUntil(ctx context.Context, started time.Time, timeout time.Duration, wantSubstr string) (status string, maxProbeLatency time.Duration, probes int) {
	deadline := started.Add(timeout)
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
		if strings.Contains(status, wantSubstr) {
			return status, maxProbeLatency, probes
		}
		time.Sleep(80 * time.Millisecond)
	}
	return status, maxProbeLatency, probes
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

// selectAndDispatchChange sets a <select>'s value and dispatches a "change"
// event — cmd/authoring-wasm's #ag-base listener is bound to "change", not
// "input" (see mountAuthoring), unlike every other field on this page.
func selectAndDispatchChange(ctx context.Context, selector, value string) error {
	return chromedp.Run(ctx,
		chromedp.SetValue(selector, value, chromedp.ByQuery),
		chromedp.EvaluateAsDevTools(
			fmt.Sprintf(`document.querySelector(%q).dispatchEvent(new Event('change', {bubbles:true}))`, selector),
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

func elementValue(ctx context.Context, selector string) (string, error) {
	var value string
	if err := chromedp.Run(ctx, chromedp.Value(selector, &value, chromedp.ByQuery)); err != nil {
		return "", err
	}
	return value, nil
}

func textContent(ctx context.Context, selector string) (string, error) {
	var value string
	if err := chromedp.Run(ctx, chromedp.Text(selector, &value, chromedp.ByQuery)); err != nil {
		return "", err
	}
	return value, nil
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
