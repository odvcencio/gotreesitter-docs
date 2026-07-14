package playground

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// TestPlaygroundBrowserSmoke is opt-in because it needs a locally installed
// Chromium. scripts/verify-production.sh runs it against the built server when
// RUN_BROWSER_SMOKE=1 (and as part of RUN_BROWSER_PERF=1).
func TestPlaygroundBrowserSmoke(t *testing.T) {
	pageURL := os.Getenv("PLAYGROUND_BROWSER_URL")
	if pageURL == "" {
		t.Skip("PLAYGROUND_BROWSER_URL is not set")
	}
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		t.Fatal(err)
	}
	rootURL := *parsedURL
	rootURL.Path = "/"
	rootURL.RawQuery = ""
	rootURL.Fragment = ""

	browserContext, cancelBrowser := chromedp.NewContext(context.Background())
	defer cancelBrowser()
	// The first visit streams a multi-megabyte parser runtime and several
	// grammars before exercising every interactive surface. Keep feature waits
	// individually bounded below, while allowing slower CI browsers enough time
	// to complete the complete lifecycle pass.
	ctx, cancel := context.WithTimeout(browserContext, 45*time.Second)
	defer cancel()

	var browserErrorsMu sync.Mutex
	var browserErrors []string
	chromedp.ListenTarget(ctx, func(event any) {
		switch typed := event.(type) {
		case *cdpruntime.EventConsoleAPICalled:
			for _, argument := range typed.Args {
				t.Logf("browser console %s: value=%s description=%s", typed.Type, argument.Value, argument.Description)
			}
		case *cdpruntime.EventExceptionThrown:
			browserErrorsMu.Lock()
			browserErrors = append(browserErrors, typed.ExceptionDetails.Text)
			browserErrorsMu.Unlock()
		}
	})

	t.Log("checking interactive playground")
	assertPlaygroundBrowser(t, ctx, pageURL)
	t.Log("checking script-disabled fallback")
	assertServerFallbackBrowser(t, browserContext, pageURL)

	// A full navigation tears down the engine realm. Returning to the route
	// proves a fresh instance can register and mount after the retained parser,
	// tree, listeners, and fetches from the first instance were disposed.
	if err := chromedp.Run(ctx,
		chromedp.Navigate(rootURL.String()),
		chromedp.WaitVisible("main", chromedp.ByQuery),
		chromedp.Navigate(pageURL),
	); err != nil {
		t.Fatal(err)
	}
	assertPlaygroundReady(t, ctx)
	waitForBrowserNodeCount(t, ctx, `[data-gosx-engine="GoTreesitterPlayground"]`, func(count int) bool { return count == 1 })
	waitForBrowserAttribute(t, ctx, "#pg-root", "data-revision", func(value string) bool { return value != "" })

	browserErrorsMu.Lock()
	errors := append([]string(nil), browserErrors...)
	browserErrorsMu.Unlock()
	if len(errors) > 0 {
		t.Fatalf("browser exceptions: %s", strings.Join(errors, "; "))
	}
}

// TestLangSearchBrowserSmoke is a separate opt-in so the dedicated standard-Go
// search engine is acceptance-tested against a fully assembled,
// version-matched GoSX runtime.
func TestLangSearchBrowserSmoke(t *testing.T) {
	pageURL := os.Getenv("LANG_SEARCH_BROWSER_URL")
	if pageURL == "" {
		t.Skip("LANG_SEARCH_BROWSER_URL is not set")
	}
	browserContext, cancelBrowser := chromedp.NewContext(context.Background())
	defer cancelBrowser()
	ctx, cancel := context.WithTimeout(browserContext, 3*time.Minute)
	defer cancel()
	var browserErrorsMu sync.Mutex
	var browserErrors []string
	chromedp.ListenTarget(ctx, func(event any) {
		switch typed := event.(type) {
		case *cdpruntime.EventConsoleAPICalled:
			for _, argument := range typed.Args {
				t.Logf("browser console %s: value=%s description=%s", typed.Type, argument.Value, argument.Description)
			}
		case *cdpruntime.EventExceptionThrown:
			browserErrorsMu.Lock()
			browserErrors = append(browserErrors, typed.ExceptionDetails.Text)
			browserErrorsMu.Unlock()
		}
	})
	defer func() {
		browserErrorsMu.Lock()
		errors := append([]string(nil), browserErrors...)
		browserErrorsMu.Unlock()
		if len(errors) > 0 {
			t.Errorf("browser exceptions: %s", strings.Join(errors, "; "))
		}
	}()
	assertLangSearchBrowser(t, ctx, pageURL)
}

// TestQueryCursorBrowserProbe advances NewQuery, Exec, and individual
// NextMatch calls in separate browser events. It is intentionally an opt-in
// promotion gate: when the WASM runtime blocks, the last completed stage names
// the exact public cursor operation that failed to return.
func TestQueryCursorBrowserProbe(t *testing.T) {
	pageURL := os.Getenv("PLAYGROUND_BROWSER_URL")
	if pageURL == "" {
		t.Skip("PLAYGROUND_BROWSER_URL is not set")
	}
	for _, probe := range []struct {
		name        string
		query       string
		wantMatches int
	}{
		{name: "root termination", query: `(source_file) @root`, wantMatches: 1},
		{name: "identifier traversal", query: `(identifier) @id`, wantMatches: 1},
		{name: "predicate traversal", query: `((identifier) @id (#match? @id "^ma"))`, wantMatches: 1},
	} {
		probe := probe
		t.Run(probe.name, func(t *testing.T) {
			matches := runQueryCursorBrowserProbe(t, pageURL, probe.query)
			if matches < probe.wantMatches {
				t.Fatalf("query returned %d matches, want at least %d", matches, probe.wantMatches)
			}
		})
	}
}

func runQueryCursorBrowserProbe(t *testing.T, pageURL, querySource string) int {
	t.Helper()
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsedURL.Query()
	query.Set("gts-query-probe", querySource)
	parsedURL.RawQuery = query.Encode()

	browserContext, cancelBrowser := chromedp.NewContext(context.Background())
	defer cancelBrowser()
	ctx, cancel := context.WithTimeout(browserContext, 20*time.Second)
	defer cancel()
	if err := chromedp.Run(ctx, chromedp.Navigate(parsedURL.String())); err != nil {
		t.Fatal(err)
	}
	assertPlaygroundReady(t, ctx)
	waitForBrowserAttribute(t, ctx, "#pg-root", "data-query-probe-stage", func(value string) bool {
		return value == "ready"
	})

	for _, want := range []string{"new-query-returned", "exec-returned"} {
		runQueryCursorBrowserStep(t, ctx, want)
	}
	for call := 1; call <= 64; call++ {
		last := browserAttribute(t, ctx, "#pg-root", "data-query-probe-stage")
		t.Logf("query cursor probe: completed=%s, invoking NextMatch call %d", last, call)
		if err := chromedp.Run(ctx,
			chromedp.Focus("#pg-qhint", chromedp.ByQuery),
			chromedp.KeyEvent(kb.Enter),
		); err != nil {
			t.Fatalf("query cursor probe blocked after %s in NextMatch call %d: %v", last, call, err)
		}
		stage := browserAttribute(t, ctx, "#pg-root", "data-query-probe-stage")
		switch stage {
		case fmt.Sprintf("next-%d-returned", call):
			runQueryCursorBrowserStep(t, ctx, fmt.Sprintf("captures-%d-returned", call))
			continue
		case fmt.Sprintf("next-%d-done", call):
			return call - 1
		default:
			t.Fatalf("NextMatch call %d ended at unexpected stage %q", call, stage)
		}
	}
	t.Fatal("query cursor did not terminate within 64 NextMatch calls")
	return 0
}

func runQueryCursorBrowserStep(t *testing.T, ctx context.Context, want string) {
	t.Helper()
	last := browserAttribute(t, ctx, "#pg-root", "data-query-probe-stage")
	t.Logf("query cursor probe: completed=%s, invoking stage expected to return %s", last, want)
	if err := chromedp.Run(ctx,
		chromedp.Focus("#pg-qhint", chromedp.ByQuery),
		chromedp.KeyEvent(kb.Enter),
	); err != nil {
		t.Fatalf("query cursor probe blocked after %s while waiting for %s: %v", last, want, err)
	}
	waitForBrowserAttribute(t, ctx, "#pg-root", "data-query-probe-stage", func(value string) bool {
		return value == want
	})
}

func assertLangSearchBrowser(t *testing.T, ctx context.Context, pageURL string) {
	t.Helper()
	if err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.WaitVisible(".lang-island", chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
	waitForBrowserAttribute(t, ctx, "html", "data-gosx-runtime-ready", func(value string) bool {
		return value == "true"
	})
	waitForBrowserAttribute(t, ctx, ".lang-island", "data-ready", func(value string) bool {
		return value == "true"
	})
	if err := chromedp.Run(ctx, chromedp.Focus(".search", chromedp.ByQuery)); err != nil {
		t.Fatal(err)
	}
	t.Log("checking complete language-search fallback")
	waitForBrowserNodeCount(t, ctx, ".lang-island", func(count int) bool { return count == 1 })
	waitForBrowserNodeCount(t, ctx, ".langtile", func(count int) bool { return count == 206 })
	waitForBrowserText(t, ctx, ".count", func(text string) bool {
		return strings.Contains(strings.Join(strings.Fields(text), " "), "206 / 206")
	})
	t.Log("checking empty language-search result")
	replaceBrowserInput(t, ctx, ".search", "definitely-not-a-language")
	if count := waitForBrowserText(t, ctx, ".count", func(text string) bool {
		return strings.Contains(strings.Join(strings.Fields(text), " "), "0 / 206")
	}); !strings.Contains(strings.Join(strings.Fields(count), " "), "0 / 206") {
		t.Fatalf("language search empty count = %q", count)
	}
	waitForBrowserNodeCount(t, ctx, ".langtile:not(.hidden)", func(count int) bool { return count == 0 })

	t.Log("checking filtered language-search result")
	replaceBrowserInput(t, ctx, ".search", "go")
	countText := waitForBrowserText(t, ctx, ".count", func(text string) bool {
		compact := strings.Join(strings.Fields(text), " ")
		return strings.HasSuffix(compact, "/ 206") && !strings.HasPrefix(compact, "0 ") && !strings.HasPrefix(compact, "206 ")
	})
	visible := waitForBrowserNodeCount(t, ctx, ".langtile:not(.hidden)", func(count int) bool {
		return count > 0 && count < 206
	})
	fields := strings.Fields(countText)
	if len(fields) != 3 || fields[1] != "/" {
		t.Fatalf("language search count has unexpected shape %q", countText)
	}
	shown, shownErr := strconv.Atoi(fields[0])
	total, totalErr := strconv.Atoi(fields[2])
	if shownErr != nil || totalErr != nil || shown != visible || total != 206 {
		t.Fatalf("language search count %q does not match %d visible / 206 total tiles", countText, visible)
	}

	replaceBrowserInput(t, ctx, ".search", "")
	waitForBrowserNodeCount(t, ctx, ".langtile:not(.hidden)", func(count int) bool { return count == 206 })
	waitForBrowserText(t, ctx, ".count", func(text string) bool {
		return strings.Contains(strings.Join(strings.Fields(text), " "), "206 / 206")
	})
}

func assertPlaygroundBrowser(t *testing.T, ctx context.Context, pageURL string) {
	t.Helper()
	if err := chromedp.Run(ctx, chromedp.Navigate(pageURL)); err != nil {
		t.Fatal(err)
	}
	assertPlaygroundReady(t, ctx)

	var options []*cdp.Node
	if err := chromedp.Run(ctx, chromedp.Nodes("#pg-lang option", &options, chromedp.ByQueryAll)); err != nil {
		t.Fatal(err)
	}
	if len(options) < 207 { // auto-detect plus the 206-grammar catalog
		t.Fatalf("language picker has %d options", len(options))
	}

	t.Log("checking python sample")
	assertPlaygroundSample(t, ctx, "python", "def fib")
	waitForBrowserNodeCount(t, ctx, "#pg-hl span", func(count int) bool { return count > 0 })

	// Return to Go through the same mounted engine before exercising queries.
	// Keeping all feature checks in one realm avoids treating repeated page
	// boot cost as feature latency; the explicit route-away/remount gate at the
	// end of TestPlaygroundBrowserSmoke owns lifecycle coverage.
	assertPlaygroundSample(t, ctx, "go", "package main")
	waitForBrowserNodeCount(t, ctx, `#pg-root[data-language="go"]`, func(count int) bool { return count == 1 })
	t.Log("checking live query and query errors")
	if err := chromedp.Run(ctx,
		chromedp.Focus("#pg-qhint", chromedp.ByQuery),
		chromedp.KeyEvent(kb.Enter),
	); err != nil {
		t.Fatal(err)
	}
	var queryValue, legendText, queryError string
	var legendHidden string
	var legendHiddenOK bool
	if err := chromedp.Run(ctx,
		chromedp.Value("#pg-query", &queryValue, chromedp.ByQuery),
		chromedp.Text("#pg-legend", &legendText, chromedp.ByQuery),
		chromedp.Text("#pg-qerr", &queryError, chromedp.ByQuery),
		chromedp.AttributeValue("#pg-legend", "hidden", &legendHidden, &legendHiddenOK, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
	t.Logf("query action returned: value=%q legend=%q hidden=%t/%q error=%q", queryValue, legendText, legendHiddenOK, legendHidden, queryError)
	waitForBrowserNodeCount(t, ctx, ".pg-legenditem", func(count int) bool { return count > 0 })
	t.Log("valid query produced captures")

	replaceBrowserInput(t, ctx, "#pg-query", "(identifier")
	if err := chromedp.Run(ctx, chromedp.WaitVisible("#pg-qerr", chromedp.ByQuery)); err != nil {
		t.Fatal(err)
	}
	t.Log("invalid query produced an error")
	// Invalid query input keeps the last valid capture result visible.
	waitForBrowserNodeCount(t, ctx, ".pg-legenditem", func(count int) bool { return count > 0 })
	if err := chromedp.Run(ctx,
		chromedp.Focus("#pg-qhint", chromedp.ByQuery),
		chromedp.KeyEvent(kb.Enter),
	); err != nil {
		t.Fatal(err)
	}
	waitForBrowserNodeCount(t, ctx, "#pg-qerr:not([hidden])", func(count int) bool { return count == 0 })
	t.Log("corrected query cleared the error")

	if count := browserNodeCount(t, ctx, ".pg-anonrow"); count != 0 {
		t.Fatalf("anonymous rows visible before opt-in: %d", count)
	}
	if err := chromedp.Run(ctx,
		chromedp.Focus("#pg-anon", chromedp.ByQuery),
		chromedp.KeyEvent(" "),
	); err != nil {
		t.Fatal(err)
	}
	waitForBrowserNodeCount(t, ctx, ".pg-anonrow", func(count int) bool { return count > 0 })

	t.Log("checking tree keyboard behavior")
	assertTreeKeyboardBrowser(t, ctx)

	for _, sample := range []struct {
		name   string
		marker string
	}{
		{name: "json", marker: `"grammars": 206`},
		{name: "markdown", marker: "A **pure Go**"},
	} {
		t.Logf("checking %s sample", sample.name)
		assertPlaygroundSample(t, ctx, sample.name, sample.marker)
	}

	t.Log("checking manual language and retained edits")
	assertManualLanguageAndIncrementalBrowser(t, ctx)
}

func assertPlaygroundSample(t *testing.T, ctx context.Context, name, marker string) {
	t.Helper()
	if err := chromedp.Run(ctx,
		chromedp.Focus(`.qchip[data-sample="`+name+`"]`, chromedp.ByQuery),
		chromedp.KeyEvent(kb.Enter),
	); err != nil {
		t.Fatal(err)
	}
	waitForBrowserValue(t, ctx, "#pg-src", func(value string) bool { return strings.Contains(value, marker) })
	waitForBrowserText(t, ctx, "#pg-langtag", func(text string) bool {
		return strings.HasPrefix(strings.TrimSpace(text), name+" ")
	})
	waitForBrowserNodeCount(t, ctx, `#pg-root[data-language="`+name+`"]`, func(count int) bool { return count == 1 })
}

func assertPlaygroundReady(t *testing.T, ctx context.Context) {
	t.Helper()
	status := waitForBrowserText(t, ctx, "#pg-stat", func(text string) bool {
		return text != "" && !strings.Contains(text, "booting") && !strings.Contains(text, "loading")
	})
	if strings.Contains(status, "unavailable") || strings.Contains(status, "failed") {
		t.Fatalf("playground failed to boot: %s", status)
	}
	if err := chromedp.Run(ctx,
		chromedp.WaitEnabled("#pg-src", chromedp.ByQuery),
		chromedp.WaitEnabled("#pg-lang", chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
}

func assertTreeKeyboardBrowser(t *testing.T, ctx context.Context) {
	t.Helper()
	var role, level string
	var roleOK, levelOK bool
	if err := chromedp.Run(ctx,
		chromedp.AttributeValue(".pg-tline[data-i]", "role", &role, &roleOK, chromedp.ByQuery),
		chromedp.AttributeValue(".pg-tline[data-i]", "aria-level", &level, &levelOK, chromedp.ByQuery),
		chromedp.Focus(".pg-tline[data-i]", chromedp.ByQuery),
		chromedp.KeyEvent(kb.End),
		chromedp.KeyEvent(kb.Home),
		chromedp.KeyEvent(kb.ArrowDown),
		chromedp.KeyEvent(kb.ArrowUp),
		chromedp.KeyEvent(kb.Enter),
	); err != nil {
		t.Fatal(err)
	}
	if !roleOK || role != "treeitem" || !levelOK || level == "" {
		t.Fatalf("tree row accessibility role=%q level=%q", role, level)
	}
	var focused []*cdp.Node
	if err := chromedp.Run(ctx, chromedp.Nodes(`.pg-tline[tabindex="0"]`, &focused, chromedp.ByQueryAll)); err != nil {
		t.Fatal(err)
	}
	if len(focused) != 1 {
		t.Fatalf("roving tree focus has %d active rows", len(focused))
	}
	var selected string
	var selectedOK bool
	if err := chromedp.Run(ctx, chromedp.AttributeValue(`.pg-tline[aria-selected="true"]`, "aria-selected", &selected, &selectedOK, chromedp.ByQuery)); err != nil {
		t.Fatal(err)
	}
	if !selectedOK || selected != "true" {
		t.Fatalf("keyboard activation did not select a tree row: %q", selected)
	}
}

func assertManualLanguageAndIncrementalBrowser(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := chromedp.Run(ctx,
		chromedp.Focus("#pg-lang", chromedp.ByQuery),
		chromedp.KeyEvent("json"),
		chromedp.KeyEvent(kb.Enter),
		chromedp.Focus(`.qchip[data-sample="json"]`, chromedp.ByQuery),
		chromedp.KeyEvent(kb.Enter),
	); err != nil {
		t.Fatal(err)
	}
	var selectedLanguage string
	if err := chromedp.Run(ctx, chromedp.Value("#pg-lang", &selectedLanguage, chromedp.ByQuery)); err != nil {
		t.Fatal(err)
	}
	if selectedLanguage != "json" {
		t.Fatalf("manual picker selected %q, want json", selectedLanguage)
	}
	waitForBrowserText(t, ctx, "#pg-langtag", func(text string) bool { return strings.TrimSpace(text) == "json" })
	waitForBrowserNodeCount(t, ctx, `#pg-root[data-language="json"]`, func(count int) bool { return count == 1 })
	revision := waitForBrowserRevision(t, ctx, 0)

	// Two same-language replacements exercise the retained parser/tree path.
	// The monotonically increasing revision proves these are updates on the
	// retained browser document rather than repeated first parses.
	for _, source := range []string{
		`{"name":"gotreesitter","grammars":207}`,
		`{"name":"gotreesitter","grammars":208}`,
	} {
		replaceBrowserInput(t, ctx, "#pg-src", source)
		waitForBrowserText(t, ctx, "#pg-hl", func(text string) bool { return strings.Contains(text, source) })
		revision = waitForBrowserRevision(t, ctx, revision)
	}
}

func waitForBrowserRevision(t *testing.T, ctx context.Context, previous uint64) uint64 {
	t.Helper()
	value := waitForBrowserAttribute(t, ctx, "#pg-root", "data-revision", func(value string) bool {
		revision, err := strconv.ParseUint(value, 10, 64)
		return err == nil && revision > previous
	})
	revision, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		t.Fatalf("invalid playground revision %q: %v", value, err)
	}
	return revision
}

func assertServerFallbackBrowser(t *testing.T, parent context.Context, pageURL string) {
	t.Helper()
	ctx, cancel := chromedp.NewContext(parent)
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, 20*time.Second)
	defer cancelTimeout()
	if err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return emulation.SetScriptExecutionDisabled(true).Do(ctx)
		}),
		chromedp.Navigate(pageURL),
		chromedp.WaitVisible("#pg-root", chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
	var source string
	if err := chromedp.Run(ctx, chromedp.Value("#pg-src", &source, chromedp.ByQuery)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(source, "package main") {
		t.Fatalf("server fallback lost initial source: %q", source)
	}
	if count := browserNodeCount(t, ctx, ".qchip[data-sample]"); count != 4 {
		t.Fatalf("server fallback sample count = %d, want 4", count)
	}
	if count := browserNodeCount(t, ctx, "#pg-lang option"); count != 1 {
		t.Fatalf("server fallback language option count = %d, want auto option only", count)
	}
	if count := browserNodeCount(t, ctx, "#pg-query"); count != 1 {
		t.Fatalf("server fallback query surface count = %d, want 1", count)
	}
	var disabled string
	var disabledOK bool
	if err := chromedp.Run(ctx, chromedp.AttributeValue("#pg-src", "disabled", &disabled, &disabledOK, chromedp.ByQuery)); err != nil {
		t.Fatal(err)
	}
	if !disabledOK {
		t.Fatal("server fallback source is not disabled before enhancement")
	}
	var role string
	var roleOK bool
	if err := chromedp.Run(ctx, chromedp.AttributeValue("#pg-tree", "role", &role, &roleOK, chromedp.ByQuery)); err != nil {
		t.Fatal(err)
	}
	if !roleOK || role != "tree" {
		t.Fatalf("server fallback tree role = %q", role)
	}
}

func replaceBrowserInput(t *testing.T, ctx context.Context, selector, value string) {
	t.Helper()
	actions := []chromedp.Action{
		chromedp.Focus(selector, chromedp.ByQuery),
		chromedp.KeyEvent("a", chromedp.KeyModifiers(input.ModifierCtrl)),
	}
	if value == "" {
		actions = append(actions, chromedp.KeyEvent(kb.Backspace))
	} else {
		actions = append(actions, chromedp.KeyEvent(value))
	}
	if err := chromedp.Run(ctx, actions...); err != nil {
		t.Fatal(err)
	}
}

func waitForBrowserText(t *testing.T, ctx context.Context, selector string, accept func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		var text string
		if err := chromedp.Run(ctx, chromedp.Text(selector, &text, chromedp.ByQuery)); err == nil {
			last = text
			if accept(text) {
				return text
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; last text %q", selector, last)
	return ""
}

func waitForBrowserValue(t *testing.T, ctx context.Context, selector string, accept func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		var value string
		if err := chromedp.Run(ctx, chromedp.Value(selector, &value, chromedp.ByQuery)); err == nil {
			last = value
			if accept(value) {
				return value
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for value of %s; last value %q", selector, last)
	return ""
}

func browserAttribute(t *testing.T, ctx context.Context, selector, name string) string {
	t.Helper()
	var value string
	var ok bool
	if err := chromedp.Run(ctx, chromedp.AttributeValue(selector, name, &value, &ok, chromedp.ByQuery)); err != nil {
		t.Fatal(err)
	}
	if !ok {
		return ""
	}
	return value
}

func waitForBrowserAttribute(t *testing.T, ctx context.Context, selector, name string, accept func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		var value string
		var ok bool
		if err := chromedp.Run(ctx, chromedp.AttributeValue(selector, name, &value, &ok, chromedp.ByQuery)); err == nil && ok {
			last = value
			if accept(value) {
				return value
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s[%s]; last value %q", selector, name, last)
	return ""
}

func waitForBrowserNodeCount(t *testing.T, ctx context.Context, selector string, accept func(int) bool) int {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		count := browserNodeCount(t, ctx, selector)
		if accept(count) {
			return count
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for node count of %s", selector)
	return 0
}

func browserNodeCount(t *testing.T, ctx context.Context, selector string) int {
	t.Helper()
	var nodes []*cdp.Node
	if err := chromedp.Run(ctx, chromedp.Nodes(selector, &nodes, chromedp.ByQueryAll, chromedp.AtLeast(0))); err != nil {
		t.Fatal(err)
	}
	return len(nodes)
}
