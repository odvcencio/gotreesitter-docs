// Command verify-playground-browser proves the playground's browser execution
// and privacy boundary against a running production server.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const privateMarker = "private_browser_only_marker"

type requestRecord struct {
	method     string
	url        string
	kind       network.ResourceType
	managedNav bool
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
		chromedp.WindowSize(1280, 900),
	)
	allocator, cancelAllocator := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAllocator()
	ctx, cancel := chromedp.NewContext(allocator)
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, 90*time.Second)
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
			requests = append(requests, requestRecord{
				method:     request.Request.Method,
				url:        request.Request.URL,
				kind:       request.Type,
				managedNav: managedNav,
			})
		}
	})

	if err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.Navigate(base+"/playground"),
		chromedp.WaitVisible("#pg-source", chromedp.ByQuery),
	); err != nil {
		fatal(err)
	}
	if err := waitForText(ctx, "#pg-status", "Parsed locally"); err != nil {
		fatal(err)
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
		if request.method != "GET" {
			fatal(fmt.Errorf("editor interaction emitted %s %s", request.method, request.url))
		}
		if strings.Contains(request.url, privateMarker) {
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
		if request.method != "GET" {
			fatal(fmt.Errorf("lazy grammar load emitted %s %s", request.method, request.url))
		}
		if strings.Contains(request.url, privateMarker) {
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
		chromedp.Focus(`a[href="/docs/getting-started"]`, chromedp.ByQuery),
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
	fmt.Println("browser verification passed: local parse, zero source egress, refresh-free navigation")
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
