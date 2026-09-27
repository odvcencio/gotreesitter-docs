// Command verify-playground-browser proves the playground's browser execution
// and privacy boundary against a running production server.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const privateMarker = "private_browser_only_marker"

var sampledLanguages = []string{
	"go", "python", "javascript", "typescript", "tsx", "rust", "c", "cpp", "java", "c_sharp",
	"ruby", "php", "kotlin", "swift", "scala", "haskell", "lua", "bash", "json", "yaml",
	"toml", "html", "css", "markdown", "sql", "elixir", "zig", "ocaml",
}

// The engine swaps its own starter sample on language changes for languages
// present in cmd/playground-wasm/samples.go. Keep fallback text here so this
// verifier does not change the playground's user-facing sample content.
var fallbackSamples = map[string]string{
	"tsx":      "const App = () => <main>Hello</main>;\n",
	"c_sharp":  "class Program { static void Main() {} }\n",
	"php":      "<?php echo \"hello\";\n",
	"kotlin":   "fun main() { println(\"hello\") }\n",
	"swift":    "let greeting = \"hello\"\n",
	"scala":    "object Main { def main(args: Array[String]): Unit = println(\"hello\") }\n",
	"haskell":  "main = putStrLn \"hello\"\n",
	"lua":      "local greeting = \"hello\"\n",
	"yaml":     "name: gotreesitter\nlanguages: 28\n",
	"toml":     "name = \"gotreesitter\"\nlanguages = 28\n",
	"html":     "<main>Hello</main>\n",
	"markdown": "# Hello\n",
	"sql":      "SELECT id, name FROM users WHERE id = 1;\n",
	"dot":      "digraph G { users -> database; }\n",
	"doxygen":  "/**\n * @brief Load a user record.\n * @param id user identifier\n * @return the matching user\n */\n",
	"dtd":      "<!ELEMENT greeting (#PCDATA)>\n",
	"ebnf":     "expression = term, { (\"+\" | \"-\"), term };\n",
	"eds":      "[app]\nname = \"gotreesitter\"\n",
	"elsa":     "let identity = \\x -> x\n",
	"facility": "service Greeting { method greet { name: string; }: { message: string; } }\n",
	"faust":    "import(\"stdfaust.lib\");\nprocess = os.osc(440) * 0.5;\n",
	"fidl":     "library example;\ntype Person = struct { name string; };\n",
	"gomod":    "module example.com/hello\ngo 1.26\n",
	"jsdoc":    "/**\n * @typedef {Object} User\n */\n",
	"mermaid":  "flowchart TD\n  Request --> Parse\n",
	"proto":    "syntax = \"proto3\";\nmessage User { string id = 1; }\n",
	"smithy":   "$version: \"2\"\nnamespace example\nservice GreetingService { version: \"1.0\", operations: [SayHello] }\noperation SayHello { input: SayHelloInput, output: SayHelloOutput }\nstructure SayHelloInput { name: String }\nstructure SayHelloOutput { greeting: String }\n",
	"tlaplus":  "---- MODULE Counter ----\nEXTENDS Naturals\nVARIABLE count\nInit == count = 0\nNext == count' = count + 1\n====\n",
	"wat":      "(module (func (export \"main\") (result i32) i32.const 42))\n",
	"devicetree": `/dts-v1/;

/ {
	model = "gotreesitter,playground";
	compatible = "gotreesitter,playground";
	#address-cells = <1>;

	memory@0 {
		device_type = "memory";
		reg = <0 0x1000>;
	};
};
`,
	"elixir": "defmodule Hello do\n  def greet, do: \"hello\"\nend\n",
	"zig":    "pub fn main() void {}\n",
	"ocaml":  "let greet name = \"hello, \" ^ name\n",
}

var realisticSamples = []string{
	"const answer = 42;\n",
	"function greet() {}\n",
	"def greet():\n    pass\n",
	"class Greeter {}\n",
	"let answer = 42;\n",
	"fn main() {}\n",
	"answer = 42\n",
	"{\"name\": \"gotreesitter\"}\n",
	"name: gotreesitter\n",
	"name = \"gotreesitter\"\n",
	"<main>Hello</main>\n",
	"# Hello\n",
	"SELECT 1;\n",
	"root { value = 1; }\n",
}

type requestRecord struct {
	method     string
	url        string
	postData   string
	kind       network.ResourceType
	managedNav bool
}

func isCloudflareBeacon(request requestRecord) bool {
	return strings.Contains(request.url, "cloudflareinsights.com/cdn-cgi/rum")
}

func carriesAny(request requestRecord, markers ...string) bool {
	data := strings.ToLower(request.url + "\n" + request.postData)
	for _, marker := range markers {
		if strings.Contains(data, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func main() {
	base := strings.TrimRight(os.Getenv("PLAYGROUND_BASE_URL"), "/")
	if base == "" {
		base = "http://127.0.0.1:18080"
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
		chromedp.Flag("mute-audio", true),
		chromedp.WindowSize(1280, 900),
	)
	allocator, cancelAllocator := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAllocator()
	ctx, cancel := chromedp.NewContext(allocator)
	defer cancel()
	timeout := 15 * time.Minute
	allSamples := strings.EqualFold(strings.TrimSpace(os.Getenv("PLAYGROUND_SAMPLE")), "all")
	if allSamples {
		timeout = 60 * time.Minute
	}
	ctx, cancelTimeout := context.WithTimeout(ctx, timeout)
	defer cancelTimeout()

	var (
		mu       sync.Mutex
		requests []requestRecord
		observe  bool
	)
	chromedp.ListenTarget(ctx, func(event any) {
		request, ok := event.(*network.EventRequestWillBeSent)
		if !ok {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if observe {
			managedNav := false
			for name := range request.Request.Headers {
				if strings.EqualFold(name, "X-GoSX-Navigation") {
					managedNav = true
					break
				}
			}
			var postData strings.Builder
			for _, entry := range request.Request.PostDataEntries {
				postData.WriteString(entry.Bytes)
			}
			requests = append(requests, requestRecord{
				method:     request.Request.Method,
				url:        request.Request.URL,
				postData:   postData.String(),
				kind:       request.Type,
				managedNav: managedNav,
			})
		}
	})

	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		fatal(err)
	}
	mu.Lock()
	observe = true
	requests = nil
	mu.Unlock()
	if err := chromedp.Run(ctx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible("h1", chromedp.ByQuery),
	); err != nil {
		fatal(err)
	}
	mu.Lock()
	homeRequests := append([]requestRecord(nil), requests...)
	requests = nil
	mu.Unlock()
	for _, request := range homeRequests {
		if strings.Contains(request.url, "/playground/runtime.wasm") || strings.Contains(request.url, "/playground/grammars/") {
			fatal(fmt.Errorf("home page requested a playground parser asset: %s", request.url))
		}
	}
	fmt.Println("home page did not request playground runtime or grammar assets")

	if err := chromedp.Run(ctx,
		chromedp.Navigate(base+"/playground?lang=rust"),
		chromedp.WaitVisible("#pg-source", chromedp.ByQuery),
	); err != nil {
		fatal(err)
	}
	if err := waitForText(ctx, "#pg-status", "Parsed locally"); err != nil {
		fatal(err)
	}
	var rustInitial struct {
		Language string `json:"language"`
		Source   string `json:"source"`
		Query    string `json:"query"`
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => ({
		language: document.querySelector("#pg-language")?.value || "",
		source: document.querySelector("#pg-source")?.value || "",
		query: document.querySelector("#pg-query")?.value || ""
	}))()`, &rustInitial)); err != nil {
		fatal(fmt.Errorf("read server-rendered Rust sample: %w", err))
	}
	if rustInitial.Language != "rust" || !strings.Contains(rustInitial.Source, "struct Greeter") || !strings.Contains(rustInitial.Query, "function_item") {
		fatal(fmt.Errorf("?lang=rust did not preselect its Rust sample: language=%q source=%q query=%q", rustInitial.Language, rustInitial.Source, rustInitial.Query))
	}
	mu.Lock()
	rustRequests := append([]requestRecord(nil), requests...)
	requests = nil
	mu.Unlock()
	for _, request := range rustRequests {
		if request.method != "GET" && !isCloudflareBeacon(request) {
			fatal(fmt.Errorf("Rust initial sample emitted %s %s", request.method, request.url))
		}
		if carriesAny(request, "Greeter", "function_item", "struct ") {
			fatal(fmt.Errorf("Rust source or query text appeared in request %s %s", request.method, request.url))
		}
	}
	fmt.Println("?lang=rust server-rendered Rust without putting source in a request")

	if err := chromedp.Run(ctx,
		chromedp.Navigate(base+"/playground"),
		chromedp.WaitVisible("#pg-source", chromedp.ByQuery),
	); err != nil {
		fatal(err)
	}
	if err := waitForText(ctx, "#pg-status", "Parsed locally"); err != nil {
		fatal(err)
	}
	var playgroundInputNames struct {
		Source   string `json:"source"`
		Query    string `json:"query"`
		Keyboard string `json:"keyboard"`
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => ({
		source: document.querySelector("#pg-source")?.getAttribute("aria-label") || "",
		query: document.querySelector("#pg-query")?.getAttribute("aria-label") || "",
		keyboard: document.querySelector('#pg-root [contenteditable="true"]')?.getAttribute("aria-label") || ""
	}))()`, &playgroundInputNames)); err != nil {
		fatal(fmt.Errorf("read playground input names: %w", err))
	}
	if playgroundInputNames.Source != "Source code" || playgroundInputNames.Query != "Tree-sitter query" || playgroundInputNames.Keyboard != "Playground keyboard input" {
		fatal(fmt.Errorf("playground input names are incomplete: source=%q query=%q keyboard=%q", playgroundInputNames.Source, playgroundInputNames.Query, playgroundInputNames.Keyboard))
	}
	var grammarOptions []*cdp.Node
	if err := chromedp.Run(ctx, chromedp.Nodes("#pg-language option", &grammarOptions, chromedp.ByQueryAll)); err != nil {
		fatal(err)
	}
	if len(grammarOptions) != 206 {
		fatal(fmt.Errorf("language picker has %d grammars; expected 206", len(grammarOptions)))
	}
	fmt.Println("browser runtime mounted and parsed the initial buffer")

	mu.Lock()
	requests = nil
	observe = true
	mu.Unlock()
	source := "package main\nfunc " + privateMarker + "() {}"
	if err := chromedp.Run(ctx,
		chromedp.SetValue("#pg-source", source, chromedp.ByQuery),
		chromedp.Click("#pg-parse", chromedp.ByQuery),
	); err != nil {
		fatal(err)
	}
	if err := waitForText(ctx, "#pg-captures", privateMarker); err != nil {
		fatal(err)
	}
	fmt.Println("private marker parsed inside the browser")

	mu.Lock()
	privacyRequests := append([]requestRecord(nil), requests...)
	requests = nil
	mu.Unlock()
	for _, request := range privacyRequests {
		if request.method != "GET" && !isCloudflareBeacon(request) {
			fatal(fmt.Errorf("editor interaction emitted %s %s", request.method, request.url))
		}
		if carriesAny(request, privateMarker) {
			fatal(fmt.Errorf("private editor marker escaped in request to %s", request.url))
		}
	}

	mu.Lock()
	requests = nil
	mu.Unlock()
	if err := chromedp.Run(ctx,
		chromedp.SetValue("#pg-language", "python", chromedp.ByQuery),
		chromedp.Click("#pg-parse", chromedp.ByQuery),
	); err != nil {
		fatal(err)
	}
	if err := waitForText(ctx, "#pg-language-label", "python"); err != nil {
		fatal(err)
	}
	mu.Lock()
	lazyGrammarRequests := append([]requestRecord(nil), requests...)
	requests = nil
	mu.Unlock()
	pythonBlobFetched := false
	for _, request := range lazyGrammarRequests {
		if request.method != "GET" && !isCloudflareBeacon(request) {
			fatal(fmt.Errorf("lazy grammar load emitted %s %s", request.method, request.url))
		}
		if carriesAny(request, privateMarker) {
			fatal(fmt.Errorf("private editor marker escaped during grammar load to %s", request.url))
		}
		if strings.Contains(request.url, "/playground/grammars/python.") && strings.HasSuffix(request.url, ".bin") {
			pythonBlobFetched = true
		}
	}
	if !pythonBlobFetched {
		fatal(fmt.Errorf("selecting Python did not lazily fetch its grammar blob; observed %#v", lazyGrammarRequests))
	}
	fmt.Println("206-language index mounted and Python grammar fetched lazily")

	clickCtx, cancelClick := context.WithTimeout(ctx, 3*time.Second)
	clickErr := chromedp.Run(clickCtx,
		chromedp.Focus(`.sidebar a[href="/docs/getting-started"]`, chromedp.ByQuery),
		chromedp.KeyEvent("\r"),
	)
	cancelClick()
	if clickErr != nil && clickErr != context.DeadlineExceeded {
		fatal(clickErr)
	}
	time.Sleep(2 * time.Second)
	mu.Lock()
	navigationRequests := append([]requestRecord(nil), requests...)
	mu.Unlock()
	managedRouteRequest := false
	for _, request := range navigationRequests {
		if request.kind == network.ResourceTypeDocument {
			fatal(fmt.Errorf("managed navigation caused a document refresh: %s", request.url))
		}
		if request.url == base+"/docs/getting-started" && request.managedNav {
			managedRouteRequest = true
		}
	}
	if !managedRouteRequest {
		fatal(fmt.Errorf("route change did not use the GoSX managed-navigation request; observed %#v", navigationRequests))
	}
	fmt.Println("managed navigation changed routes without a document refresh")
	mu.Lock()
	observe = false
	requests = nil
	mu.Unlock()
	if err := chromedp.Run(ctx,
		chromedp.Navigate(base+"/playground"),
		chromedp.WaitVisible("#pg-language", chromedp.ByQuery),
	); err != nil {
		fatal(fmt.Errorf("reopen playground for language samples: %w", err))
	}
	if err := waitForText(ctx, "#pg-status", "Parsed locally"); err != nil {
		fatal(fmt.Errorf("wait for playground after managed-navigation check: %w", err))
	}

	languages := sampledLanguages
	if allSamples {
		if err := chromedp.Run(ctx, chromedp.Evaluate(`Array.from(document.querySelectorAll("#pg-language option")).map(option => option.value)`, &languages)); err != nil {
			fatal(fmt.Errorf("read all language picker values: %w", err))
		}
	}
	failedLanguages := 0
	for _, language := range languages {
		started := time.Now()
		if err := loadLanguageSample(ctx, language); err != nil {
			failedLanguages++
			fmt.Printf("language %s FAIL %dms: %v\n", language, time.Since(started).Milliseconds(), err)
			continue
		}
		treeTimeout := 25 * time.Second
		if allSamples {
			treeTimeout = 8 * time.Second
		}
		state, err := waitForLanguageTree(ctx, language, treeTimeout)
		elapsed := time.Since(started).Milliseconds()
		if err != nil {
			failedLanguages++
			fmt.Printf("language %s FAIL %dms: %v\n", language, elapsed, err)
			continue
		}
		if allSamples && fallbackSamples[language] == "" {
			var source string
			if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector("#pg-source")?.value || ""`, &source)); err != nil {
				failedLanguages++
				fmt.Printf("language %s FAIL %dms: read sample source: %v\n", language, time.Since(started).Milliseconds(), err)
				continue
			}
			if strings.TrimSpace(source) == "" {
				state, err = tryRealisticSamples(ctx, language, treeTimeout)
				elapsed = time.Since(started).Milliseconds()
				if err != nil {
					failedLanguages++
					fmt.Printf("language %s FAIL %dms: %v\n", language, elapsed, err)
					continue
				}
			}
		}
		if !state.Root {
			failedLanguages++
			fmt.Printf("language %s FAIL %dms: tree has no root node (status=%q rows=%d text=%q errors=%q)\n", language, elapsed, state.Status, state.Rows, state.TreeText, state.Errors)
			continue
		}
		if state.BadNode {
			failedLanguages++
			fmt.Printf("language %s FAIL %dms: tree contains ERROR or MISSING node\n", language, elapsed)
			continue
		}
		fmt.Printf("language %s PASS %dms\n", language, elapsed)
		if language == "sql" {
			started = time.Now()
			state, err = parseSampleSource(ctx, language, "SELECT $$hello$$;\n", treeTimeout)
			elapsed = time.Since(started).Milliseconds()
			if err != nil {
				failedLanguages++
				fmt.Printf("language sql PostgreSQL dollar quote FAIL %dms: %v\n", elapsed, err)
			} else if !state.Root || state.BadNode {
				failedLanguages++
				fmt.Printf("language sql PostgreSQL dollar quote FAIL %dms: tree has no clean root (status=%q rows=%d errors=%q)\n", elapsed, state.Status, state.Rows, state.Errors)
			} else {
				fmt.Printf("language sql PostgreSQL dollar quote PASS %dms\n", elapsed)
			}
		}
	}
	if failedLanguages != 0 {
		fatal(fmt.Errorf("%d of %d playground language samples failed", failedLanguages, len(languages)))
	}
	fmt.Println("browser verification passed: local parse, zero source egress, refresh-free navigation")
}

type languageTreeState struct {
	Language string `json:"language"`
	Status   string `json:"status"`
	Root     bool   `json:"root"`
	BadNode  bool   `json:"badNode"`
	Rows     int    `json:"rows"`
	TreeText string `json:"treeText"`
	Errors   string `json:"errors"`
}

func loadLanguageSample(ctx context.Context, language string) error {
	source := ""
	if fallback, ok := fallbackSamples[language]; ok {
		source = fallback
	}
	script := `(() => {
		const picker = document.querySelector("#pg-language");
		const editor = document.querySelector("#pg-source");
		const query = document.querySelector("#pg-query");
		picker.value = ` + strconv.Quote(language) + `;
		editor.value = ` + strconv.Quote(source) + `;
		query.value = "";
		picker.dispatchEvent(new Event("change", { bubbles: true }));
	})()`
	return chromedp.Run(ctx, chromedp.Evaluate(script, nil))
}

func tryRealisticSamples(ctx context.Context, language string, timeout time.Duration) (languageTreeState, error) {
	var state languageTreeState
	for _, source := range realisticSamples {
		var err error
		state, err = parseSampleSource(ctx, language, source, timeout)
		if err != nil {
			return state, err
		}
		if state.Root && !state.BadNode {
			return state, nil
		}
	}
	return state, fmt.Errorf("no realistic statement or declaration parsed without ERROR or MISSING nodes")
}

func parseSampleSource(ctx context.Context, language, source string, timeout time.Duration) (languageTreeState, error) {
	script := `(() => {
		const picker = document.querySelector("#pg-language");
		const editor = document.querySelector("#pg-source");
		const query = document.querySelector("#pg-query");
		picker.value = ` + strconv.Quote(language) + `;
		editor.value = ` + strconv.Quote(source) + `;
		query.value = "";
		editor.dispatchEvent(new Event("input", { bubbles: true }));
		picker.dispatchEvent(new Event("change", { bubbles: true }));
		document.querySelector("#pg-parse").click();
	})()`
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, nil)); err != nil {
		return languageTreeState{}, err
	}
	return waitForLanguageTree(ctx, language, timeout)
}

func waitForLanguageTree(ctx context.Context, language string, timeout time.Duration) (languageTreeState, error) {
	deadline := time.Now().Add(timeout)
	script := `(() => {
		const rows = Array.from(document.querySelectorAll("#pg-tree [role=treeitem]"));
		const label = document.querySelector("#pg-language-label");
		const status = document.querySelector("#pg-status");
		return {
			language: label ? label.textContent.trim() : "",
			status: status ? status.textContent.trim() : "",
			root: rows.some(row => row.getAttribute("aria-level") === "1"),
			rows: rows.length,
			treeText: document.querySelector("#pg-tree")?.textContent.trim() || "",
			errors: document.querySelector("#pg-errors")?.textContent.trim() || "",
			badNode: rows.some(row => row.classList.contains("pg-err") ||
				(Array.from(row.querySelectorAll(".ttype")).some(node => node.textContent.trim() === "ERROR")) ||
				row.querySelector(".tmissing") !== null)
		};
	})()`
	for time.Now().Before(deadline) {
		var state languageTreeState
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &state)); err != nil {
			return state, err
		}
		if state.Language == language && strings.HasPrefix(state.Status, "Parsed locally") {
			return state, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return languageTreeState{}, fmt.Errorf("tree for %s did not finish within %s", language, timeout)
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
	if configured := os.Getenv("PLAYGROUND_CHROME"); configured != "" {
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
	fmt.Fprintln(os.Stderr, "verify-playground-browser:", err)
	os.Exit(1)
}
