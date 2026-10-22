package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/odvcencio/gotreesitter-docs/internal/playgroundshare"
	runtimehost "m31labs.dev/gosx/client/runtime/host"
)

func TestVersionedPublicAssetURL(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "public", "playground", "playground.css")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(".playground{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	url := versionedPublicAssetURL(root, "playground/playground.css")
	if !strings.HasPrefix(url, "/playground/playground.css?v=") {
		t.Fatalf("versioned asset URL = %q", url)
	}
}

func TestMissingPublicAssetUsesFrameworkURL(t *testing.T) {
	if got := versionedPublicAssetURL(t.TempDir(), "missing.css"); got != "/missing.css" {
		t.Fatalf("missing asset URL = %q", got)
	}
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	app, err := newApp(filepath.Dir(thisFile))
	if err != nil {
		t.Fatal(err)
	}
	return app.Build()
}

func get(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestPlaygroundServesWithAndWithoutTrailingSlash(t *testing.T) {
	handler := testHandler(t)
	for _, path := range []string{"/playground", "/playground/"} {
		rec := get(t, handler, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `data-gosx-engine="GotreesitterPlayground"`) {
			t.Fatalf("GET %s did not render the playground surface", path)
		}
	}
}

func TestPagesLoadTheNavigationRuntimeFromGoSXHashedAsset(t *testing.T) {
	handler := testHandler(t)
	body := get(t, handler, "/docs/introduction").Body.String()
	match := regexp.MustCompile(`<script[^>]*data-gosx-navigation="true"[^>]*src="([^"]+)"`).FindStringSubmatch(body)
	if match == nil {
		t.Fatal("no navigation runtime script tag on /docs/introduction")
	}
	if match[1] != runtimehost.NavigationRuntimePath {
		t.Fatalf("navigation src = %q, want %q", match[1], runtimehost.NavigationRuntimePath)
	}
	if strings.Contains(body, "/_app/runtime/navigation.") {
		t.Fatal("the legacy re-served runtime URL is still rendered")
	}
	asset := get(t, handler, match[1])
	if asset.Code != http.StatusOK {
		t.Fatalf("GET %s = %d", match[1], asset.Code)
	}
	if ct := asset.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Fatalf("content-type = %q", ct)
	}
	if cc := asset.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("cache-control = %q, want immutable", cc)
	}
	if asset.Body.String() != runtimehost.NavigationRuntime {
		t.Fatal("served runtime differs from the embedded GoSX runtime")
	}
}

func TestPlaygroundDeepLinkRendersTheNamedSample(t *testing.T) {
	handler := testHandler(t)
	body := get(t, handler, "/playground?lang=python&sample=add").Body.String()
	for _, want := range []string{`data-sample="add"`, "def add(left, right)"} {
		if !strings.Contains(body, want) {
			t.Errorf("deep link lacks %q", want)
		}
	}
	if body := get(t, handler, "/playground?lang=python&sample=nope").Body.String(); !strings.Contains(body, `data-sample="greet"`) {
		t.Error("unknown sample name must resolve to greet")
	}
}

func TestPlaygroundShareLinkRendering(t *testing.T) {
	handler := testHandler(t)
	// "func shared_marker_only() {}" encoded the way the browser does.
	values, err := playgroundshare.Encode(playgroundshare.Share{Language: "go", Source: "package main\nfunc shared_marker_only() {}\n"})
	if err != nil {
		t.Fatal(err)
	}
	body := get(t, handler, "/playground?"+values.Encode()).Body.String()
	if !strings.Contains(body, "shared_marker_only") || !strings.Contains(body, `data-shared="true"`) {
		t.Errorf("a valid share link must open its source with data-shared=true")
	}
	if strings.Contains(body, "pg-share-note") {
		t.Error("a valid share link must not show the error notice")
	}
	rec := get(t, handler, "/playground?lang=go&s=1.%25%25%25")
	if rec.Code != http.StatusOK {
		t.Fatalf("a broken share link must still open the playground, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "This share link could not be opened") || !strings.Contains(rec.Body.String(), `data-shared="false"`) {
		t.Error("a broken share link must show the notice and data-shared=false")
	}
	plain := get(t, handler, "/playground").Body.String()
	if strings.Contains(plain, "pg-share-note") || !strings.Contains(plain, `id="pg-share"`) {
		t.Error("the plain playground shows the share button and no notice")
	}
}

func TestDocsPagesDoNotLoadTheLiteBootstrap(t *testing.T) {
	// The copy buttons need GoSX's lite bootstrap (about 151 KB decoded), which
	// the docs JS budget cannot carry until GoSX ships a standalone copy chunk.
	body := get(t, testHandler(t), "/docs/getting-started").Body.String()
	if strings.Contains(body, "bootstrap-lite") || strings.Contains(body, `data-gosx-copy-button`) {
		t.Fatal("docs pages must not load the lite bootstrap or render copy buttons")
	}
}

func TestNotFoundPagesLeadToSearchAndThePlayground(t *testing.T) {
	handler := testHandler(t)
	for _, path := range []string{"/this-page-does-not-exist", "/docs/this-doc-does-not-exist"} {
		rec := get(t, handler, path)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404", path, rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{`action="/changelog"`, `name="q"`, `href="/playground?lang=go&amp;sample=add"`, `href="/docs/getting-started"`} {
			if !strings.Contains(body, want) {
				t.Errorf("GET %s lacks %q", path, want)
			}
		}
	}
}

func TestLanguagesFilterRunsOnTheServer(t *testing.T) {
	handler := testHandler(t)
	all := get(t, handler, "/docs/languages").Body.String()
	if !strings.Contains(all, `name="q"`) || strings.Contains(all, "LangSearch") {
		t.Fatal("languages page must render the GET filter form and no island")
	}
	filtered := get(t, handler, "/docs/languages?q=script").Body.String()
	if !strings.Contains(filtered, "lname\">typescript<") || strings.Contains(filtered, "lname\">python<") {
		t.Fatal("?q=script must keep typescript and drop python")
	}
	if !strings.Contains(filtered, `value="script"`) {
		t.Fatal("the filter box must keep the query")
	}
	if none := get(t, handler, "/docs/languages?q=zzzz").Body.String(); !strings.Contains(none, "No language matches") {
		t.Fatal("an empty result must say so")
	}
}
