// Pre-deploy hardening (the worker watchdog vs. the shipped base grammars —
// see cmd/authoring-wasm's hardBudget doc and cmd/build-authoring-wasm's
// baseGrammars doc): this file adds base-agnostic instrumentation for
// measuring a base's REAL worker-side (wasm) compile time and for asserting
// a base compiles within a given budget with zero watchdog kill+respawn
// cycles. It backs three things:
//
//   - the one-off measurement that decided cmd/authoring-wasm's hardBudget
//     and the final public/authoring/bases/index.json base list (go must
//     have comfortable headroom; typescript/javascript are kept only if they
//     fit the raised budget, dropped otherwise);
//   - checkGoBaseNoRespawns: the permanent regression check that the
//     headline "inherit Go" demo never flaps (kill+respawn) under normal
//     conditions;
//   - checkAllBasesWithinBudget: the permanent regression check that every
//     base CURRENTLY listed in bases/index.json (whatever that list is —
//     this reads the live #ag-base options, it does not hardcode names)
//     compiles within budget with zero respawns. A future edit to
//     baseGrammars is automatically covered without touching this file.
package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// reParsedIn matches cmd/authoring-wasm render()'s success-with-no-parse-
// errors status: `Compiled "name" and parsed in <duration> (<n> conflicts).`
var reParsedIn = regexp.MustCompile(`parsed in (.+?) \(`)

// reParseErrorsElapsed matches render()'s HasErrors status (the sample
// parsed with recoverable error nodes, not a hard ParseError):
// `Compiled "name"; sample has parse errors (<duration>).`
var reParseErrorsElapsed = regexp.MustCompile(`parse errors \((.+?)\)\.`)

// parseElapsedFromStatus extracts the worker's own self-reported elapsedMs
// (see cmd/authoring-worker-wasm's postMessage — the actual wasm-side
// generate+diagnose+parse cost, exactly what cmd/authoring-wasm's hardBudget
// must budget for) out of an #ag-status string, when that status renders it.
// A handful of render() branches (ImportError/GenerateError/TimedOut/hard
// ParseError) do not include an elapsed duration at all; ok is false there.
func parseElapsedFromStatus(status string) (elapsed time.Duration, ok bool) {
	if m := reParsedIn.FindStringSubmatch(status); m != nil {
		if d, err := time.ParseDuration(m[1]); err == nil {
			return d, true
		}
	}
	if m := reParseErrorsElapsed.FindStringSubmatch(status); m != nil {
		if d, err := time.ParseDuration(m[1]); err == nil {
			return d, true
		}
	}
	return 0, false
}

// baseTiming is one measureBaseCompile observation.
type baseTiming struct {
	// WallClock is the time THIS PROCESS observed from switching #ag-base to
	// the terminal status appearing — includes any watchdog kill+respawn
	// cycles, Worker boot time (amortized after the first base of a page
	// load), and postMessage/event-loop scheduling. Not the same thing as a
	// single compile's own cost — see WorkerElapsed for that.
	WallClock time.Duration
	// WorkerElapsed is the worker's own self-reported elapsedMs for
	// whichever attempt finally produced the terminal status, parsed out of
	// #ag-status (see parseElapsedFromStatus). Zero/Valid=false if the
	// terminal status doesn't carry one (ImportError/GenerateError/TimedOut,
	// or the base never reached a terminal status before the timeout).
	WorkerElapsed      time.Duration
	WorkerElapsedValid bool
	// Respawns counts how many times cmd/authoring-wasm's onHardTimeout
	// killed and restarted the worker while this measurement was waiting —
	// the direct signal for "hardBudget is too tight for this base".
	Respawns int
	// Completed is true iff a `Compiled "<name>"` status (success OR a soft
	// ParseError — both start with that prefix; see render()) was observed
	// before the timeout. False means the base never finished — under a
	// fixed hardBudget that is a permanent kill/respawn loop, not a slow but
	// eventually-successful compile: every retry starts generation over from
	// scratch, so if one attempt's wasm-side cost exceeds hardBudget, EVERY
	// attempt will too.
	Completed   bool
	FinalStatus string
}

// measureBaseCompile switches #ag-base to name (after clearing the delta to
// "{}" and setting sample as the sample source — set BEFORE the base switch,
// same race-avoidance ordering as checkExplicitBaseSwitch/checkGoBase: once
// the base's grammar.json fetch lands, the auto-triggered run() reads
// whatever is currently in the editors), then polls #ag-status up to timeout
// for a `Compiled "<name>"` terminal status, tracking every transition into
// cmd/authoring-wasm's "restarting the background compiler" status as one
// respawn. It never calls fatal() itself — callers decide what a given
// observation means (a diagnostic measurement pass tolerates
// Completed==false; a regression assertion does not).
func measureBaseCompile(ctx context.Context, name, sample string, timeout time.Duration) baseTiming {
	if err := setValueAndDispatchInput(ctx, "#ag-grammar", "{}"); err != nil {
		fatal(err)
	}
	if err := setValueAndDispatchInput(ctx, "#ag-source", sample); err != nil {
		fatal(err)
	}

	started := time.Now()
	if err := selectAndDispatchChange(ctx, "#ag-base", name); err != nil {
		fatal(err)
	}

	wantSubstr := fmt.Sprintf("Compiled %q", name)
	deadline := started.Add(timeout)
	var status string
	respawns := 0
	wasRestarting := false
	completed := false
	for time.Now().Before(deadline) {
		if err := chromedp.Run(ctx, chromedp.Text("#ag-status", &status, chromedp.ByQuery)); err != nil {
			fatal(err)
		}
		isRestarting := strings.Contains(status, "restarting the background compiler")
		if isRestarting && !wasRestarting {
			respawns++
		}
		wasRestarting = isRestarting
		if strings.Contains(status, wantSubstr) {
			completed = true
			break
		}
		time.Sleep(80 * time.Millisecond)
	}

	timing := baseTiming{
		WallClock:   time.Since(started),
		Respawns:    respawns,
		Completed:   completed,
		FinalStatus: status,
	}
	if completed {
		timing.WorkerElapsed, timing.WorkerElapsedValid = parseElapsedFromStatus(status)
	}
	return timing
}

// measureAndReport runs measureBaseCompile and prints a one-line summary —
// used by both the diagnostic (pre-decision) measurement pass and the
// permanent regression checks below, so their output is directly
// comparable.
func measureAndReport(ctx context.Context, label, name, sample string, timeout time.Duration) baseTiming {
	t := measureBaseCompile(ctx, name, sample, timeout)
	switch {
	case !t.Completed:
		fmt.Printf("%s: base %-12s DID NOT COMPLETE within %s (wall-clock %s, %d respawn(s) observed, last status %q)\n",
			label, name, timeout, t.WallClock.Round(time.Millisecond), t.Respawns, truncate(t.FinalStatus, 160))
	case t.WorkerElapsedValid:
		fmt.Printf("%s: base %-12s worker-reported wasm compile %s, wall-clock %s, %d respawn(s)\n",
			label, name, t.WorkerElapsed.Round(time.Millisecond), t.WallClock.Round(time.Millisecond), t.Respawns)
	default:
		fmt.Printf("%s: base %-12s completed (no elapsed in status: %q), wall-clock %s, %d respawn(s)\n",
			label, name, truncate(t.FinalStatus, 160), t.WallClock.Round(time.Millisecond), t.Respawns)
	}
	return t
}

// listBaseOptions reads the live #ag-base <option> values (excluding the
// blank "full grammar" option) — i.e. whatever public/authoring/bases/
// index.json actually shipped for THIS build, not a hardcoded name list, so
// a future edit to cmd/build-authoring-wasm's baseGrammars is automatically
// covered by checkAllBasesWithinBudget without touching this file.
func listBaseOptions(ctx context.Context) []string {
	var names []string
	if err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(
		`Array.from(document.querySelectorAll('#ag-base option')).map(o => o.value).filter(v => v !== '')`,
		&names,
	)); err != nil {
		fatal(err)
	}
	return names
}

// checkGoBaseNoRespawns is the pre-deploy hardening regression check for the
// headline "inherit Go" demo (task item 3): compiles the go base repeatedly
// under normal (single-request-at-a-time, no artificial load) conditions and
// asserts the main-thread watchdog (cmd/authoring-wasm's hardBudget) never
// kills+respawns the worker. Each iteration bounces #ag-base through calc
// first so returning to "go" is a genuine base-switch + recompile (a real
// fetch+merge+compile cycle), not a debounce no-op against an already-
// selected value.
func checkGoBaseNoRespawns(ctx context.Context, iterations int, perAttemptBudget time.Duration) {
	for i := 1; i <= iterations; i++ {
		if err := setValueAndDispatchInput(ctx, "#ag-grammar", "{}"); err != nil {
			fatal(err)
		}
		if err := selectAndDispatchChange(ctx, "#ag-base", "calc"); err != nil {
			fatal(err)
		}
		if err := waitForText(ctx, "#ag-status", `Compiled "calc"`); err != nil {
			fatal(fmt.Errorf("go-base repeat check %d/%d: switch to calc: %w", i, iterations, err))
		}

		t := measureAndReport(ctx, fmt.Sprintf("go-base repeat %d/%d", i, iterations), "go", goSample, perAttemptBudget)
		if !t.Completed {
			fatal(fmt.Errorf("go-base repeat check %d/%d: did not complete within %s (last status %q) — hardBudget is not comfortably above go's measured wasm compile time", i, iterations, perAttemptBudget, t.FinalStatus))
		}
		if t.Respawns != 0 {
			fatal(fmt.Errorf("go-base repeat check %d/%d: worker was killed+respawned %d time(s) — hardBudget is still too tight for the go base under normal conditions", i, iterations, t.Respawns))
		}
	}
	fmt.Printf("go base compiled %d/%d times with zero watchdog kill/respawn cycles\n", iterations, iterations)
}

// checkAllBasesWithinBudget is the pre-deploy hardening regression check for
// task item 3's "assert every base in the final bases/index.json compiles
// within budget": sweeps every base currently listed in #ag-base (i.e.
// whatever this build's bases/index.json actually ships), asserting each
// reaches a terminal `Compiled "<name>"` status within perBaseBudget with
// zero watchdog respawns — the "don't ship a base that flaps or never
// finishes" principle (design Phase 2, re-affirmed by this hardening pass)
// applies to the whole shipped catalog, not just go.
func checkAllBasesWithinBudget(ctx context.Context, perBaseBudget time.Duration) {
	names := listBaseOptions(ctx)
	if len(names) == 0 {
		fatal(fmt.Errorf("checkAllBasesWithinBudget: #ag-base has no non-blank options — bases/index.json did not load"))
	}
	for _, name := range names {
		t := measureAndReport(ctx, "budget sweep", name, "", perBaseBudget)
		if !t.Completed {
			fatal(fmt.Errorf("base %q did not compile within the %s budget (last status %q) — should not be shipped at the current hardBudget, or hardBudget is too tight for it", name, perBaseBudget, t.FinalStatus))
		}
		if t.Respawns != 0 {
			fatal(fmt.Errorf("base %q was killed+respawned %d time(s) while compiling within the %s budget window — should not be shipped at the current hardBudget", name, t.Respawns, perBaseBudget))
		}
	}
	fmt.Printf("every shipped base (%v) compiled within the %s budget with zero watchdog respawns\n", names, perBaseBudget)
}
