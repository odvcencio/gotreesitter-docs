package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	"m31labs.dev/gosx"
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

func TestDeferredNavigationHeadUsesCacheableAssetAndKeepsNonce(t *testing.T) {
	got := gosx.RenderHTML(deferredNavigationHead("test-nonce"))
	for _, want := range []string{
		"defer",
		`data-gosx-navigation="true"`,
		`fetchpriority="low"`,
		`src="` + navigationRuntimeAssetURL() + `"`,
		`nonce="test-nonce"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("deferred navigation head does not contain %q", want)
		}
	}
	if strings.Contains(got, runtimehost.NavigationRuntime) {
		t.Fatal("navigation runtime is still embedded in the document head")
	}
}

func TestNavigationRuntimeAssetSupportsBrotliAndImmutableCaching(t *testing.T) {
	app := server.New()
	app.Mount(navigationRuntimeAssetURL(), navigationRuntimeAssetHandler())
	request := httptest.NewRequest(http.MethodGet, navigationRuntimeAssetURL(), nil)
	request.Header.Set("Accept-Encoding", "gzip, br")
	response := httptest.NewRecorder()
	app.Build().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("navigation runtime returned %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Encoding") != "br" {
		t.Fatalf("navigation runtime encoding = %q", response.Header().Get("Content-Encoding"))
	}
	if response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("navigation runtime cache policy = %q", response.Header().Get("Cache-Control"))
	}
	decoded, err := io.ReadAll(brotli.NewReader(bytes.NewReader(response.Body.Bytes())))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, []byte(runtimehost.NavigationRuntime)) {
		t.Fatal("navigation runtime response differs from GoSX's embedded runtime")
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
	mountIslandProgram(app, docsapp.LangSearchProgramPath, program, docsapp.LangSearchProgramContentVersion())
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
