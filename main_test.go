package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	runtimehost "m31labs.dev/gosx/client/runtime/host"
	islandprogram "m31labs.dev/gosx/island/program"
	"m31labs.dev/gosx/server"
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

func TestLangSearchIslandIsServedOutsideGoSXRuntimePaths(t *testing.T) {
	if strings.HasPrefix(docsapp.LangSearchProgramPath, "/gosx/") {
		t.Fatalf("LangSearch program path collides with GoSX runtime paths: %q", docsapp.LangSearchProgramPath)
	}
	program := docsapp.LangSearchProgram()
	want, err := islandprogram.EncodeJSON(program)
	if err != nil {
		t.Fatal(err)
	}

	app := server.New()
	app.SetRuntimeRoot(t.TempDir())
	if err := mountIslandProgram(app, docsapp.LangSearchProgramPath, program, docsapp.LangSearchProgramContentVersion()); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, docsapp.LangSearchProgramURL(), nil)
	app.Build().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("LangSearch program returned %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("LangSearch content type = %q", got)
	}
	if !bytes.Equal(response.Body.Bytes(), want) {
		t.Fatalf("LangSearch program response is not the JSON program payload")
	}
}

// testHandler builds the production router against this repository root.
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
