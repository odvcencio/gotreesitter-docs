package main

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const testReleaseVersion = "v0.33.0"

func TestPlaygroundRuntimeHandlerNegotiatesAcceptEncoding(t *testing.T) {
	root := t.TempDir()
	raw := []byte("\x00asm gotreesitter runtime")
	gzipped := writeRuntimeFixture(t, root, raw, true, true)
	handler := playgroundRuntimeHandler(root, testReleaseVersion)

	tests := []struct {
		name                 string
		acceptEncodingValues []string
		wantStatus           int
		wantEncoding         string
		wantBody             []byte
	}{
		{name: "absent header uses identity", wantStatus: http.StatusOK, wantBody: raw},
		{name: "present empty header uses identity", acceptEncodingValues: []string{""}, wantStatus: http.StatusOK, wantBody: raw},
		{name: "gzip wins implicit tie", acceptEncodingValues: []string{"br, gzip"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: gzipped},
		{name: "split field lines are combined", acceptEncodingValues: []string{"identity;q=0.2", "gzip;q=0.8"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: gzipped},
		{name: "x-gzip alias", acceptEncodingValues: []string{"x-gzip"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: gzipped},
		{name: "identity has higher quality", acceptEncodingValues: []string{"identity;q=0.9, gzip;q=0.4"}, wantStatus: http.StatusOK, wantBody: raw},
		{name: "gzip has higher quality", acceptEncodingValues: []string{"identity;q=0.2, gzip;q=0.8"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: gzipped},
		{name: "gzip q zero", acceptEncodingValues: []string{"*;q=1, gzip;q=0"}, wantStatus: http.StatusOK, wantBody: raw},
		{name: "identity q zero", acceptEncodingValues: []string{"identity;q=0, gzip;q=0.5"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: gzipped},
		{name: "wildcard tie prefers gzip", acceptEncodingValues: []string{"*;q=1"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: gzipped},
		{name: "wildcard lower than default identity", acceptEncodingValues: []string{"*;q=0.5"}, wantStatus: http.StatusOK, wantBody: raw},
		{name: "identity overrides disabled wildcard", acceptEncodingValues: []string{"*;q=0, identity;q=0.5"}, wantStatus: http.StatusOK, wantBody: raw},
		{name: "gzip overrides disabled wildcard", acceptEncodingValues: []string{"*;q=0, gzip;q=0.5"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: gzipped},
		{name: "wildcard disables every representation", acceptEncodingValues: []string{"*;q=0"}, wantStatus: http.StatusNotAcceptable},
		{name: "both explicit qualities zero", acceptEncodingValues: []string{"identity;q=0, gzip;q=0"}, wantStatus: http.StatusNotAcceptable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/playground/runtime.wasm?v="+testReleaseVersion, nil)
			if test.acceptEncodingValues != nil {
				request.Header["Accept-Encoding"] = append([]string(nil), test.acceptEncodingValues...)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if got := response.Header().Values("Vary"); !headerValuesContain(got, "Accept-Encoding") {
				t.Fatalf("Vary = %q", got)
			}
			if test.wantStatus != http.StatusOK {
				return
			}
			if got := response.Header().Get("Content-Type"); got != "application/wasm" {
				t.Fatalf("Content-Type = %q", got)
			}
			if got := response.Header().Get("Content-Encoding"); got != test.wantEncoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, test.wantEncoding)
			}
			if !bytes.Equal(response.Body.Bytes(), test.wantBody) {
				t.Fatalf("body = %d bytes, want %d", response.Body.Len(), len(test.wantBody))
			}
		})
	}
}

func TestPlaygroundRuntimeHandlerFallsBackToAvailableRepresentation(t *testing.T) {
	raw := []byte("\x00asm fallback runtime")
	tests := []struct {
		name                 string
		withRaw              bool
		withGzip             bool
		acceptEncodingValues []string
		wantStatus           int
		wantEncoding         string
		wantGzipBody         bool
	}{
		{
			name:                 "missing preferred gzip uses identity",
			withRaw:              true,
			acceptEncodingValues: []string{"identity;q=0.2, gzip;q=1"},
			wantStatus:           http.StatusOK,
		},
		{
			name:                 "missing preferred identity uses gzip",
			withGzip:             true,
			acceptEncodingValues: []string{"identity;q=1, gzip;q=0.2"},
			wantStatus:           http.StatusOK,
			wantEncoding:         "gzip",
			wantGzipBody:         true,
		},
		{
			name:                 "only unavailable representation acceptable",
			withRaw:              true,
			acceptEncodingValues: []string{"identity;q=0, gzip;q=1"},
			wantStatus:           http.StatusNotAcceptable,
		},
		{
			name:         "absent header accepts gzip-only representation",
			withGzip:     true,
			wantStatus:   http.StatusOK,
			wantEncoding: "gzip",
			wantGzipBody: true,
		},
		{
			name:                 "present empty header rejects gzip-only representation",
			withGzip:             true,
			acceptEncodingValues: []string{""},
			wantStatus:           http.StatusNotAcceptable,
		},
		{
			name:                 "no representation exists",
			acceptEncodingValues: []string{"gzip"},
			wantStatus:           http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			gzipped := writeRuntimeFixture(t, root, raw, test.withRaw, test.withGzip)
			handler := playgroundRuntimeHandler(root, testReleaseVersion)
			request := httptest.NewRequest(http.MethodGet, "/playground/runtime.wasm?v="+testReleaseVersion, nil)
			if test.acceptEncodingValues != nil {
				request.Header["Accept-Encoding"] = append([]string(nil), test.acceptEncodingValues...)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantStatus != http.StatusOK {
				return
			}
			if got := response.Header().Get("Content-Encoding"); got != test.wantEncoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, test.wantEncoding)
			}
			wantBody := raw
			if test.wantGzipBody {
				wantBody = gzipped
			}
			if !bytes.Equal(response.Body.Bytes(), wantBody) {
				t.Fatalf("body = %d bytes, want %d", response.Body.Len(), len(wantBody))
			}
		})
	}
}

func TestPlaygroundRuntimeHandlerHeadConditionalAndCacheSemantics(t *testing.T) {
	root := t.TempDir()
	raw := []byte("\x00asm cache runtime")
	gzipped := writeRuntimeFixture(t, root, raw, true, true)
	handler := playgroundRuntimeHandler(root, testReleaseVersion)

	getRequest := httptest.NewRequest(http.MethodGet, "/playground/runtime.wasm?v="+testReleaseVersion, nil)
	getRequest.Header.Set("Accept-Encoding", "gzip")
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("GET status = %d", getResponse.Code)
	}
	if got := getResponse.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
	etag := getResponse.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag is empty")
	}

	headRequest := httptest.NewRequest(http.MethodHead, "/playground/runtime.wasm?v="+testReleaseVersion, nil)
	headRequest.Header.Set("Accept-Encoding", "gzip")
	headResponse := httptest.NewRecorder()
	handler.ServeHTTP(headResponse, headRequest)
	if headResponse.Code != http.StatusOK {
		t.Fatalf("HEAD status = %d", headResponse.Code)
	}
	if headResponse.Body.Len() != 0 {
		t.Fatalf("HEAD body = %d bytes", headResponse.Body.Len())
	}
	if got := headResponse.Header().Get("Content-Length"); got != strconv.Itoa(len(gzipped)) {
		t.Fatalf("HEAD Content-Length = %q, want %d", got, len(gzipped))
	}

	conditionalRequest := httptest.NewRequest(http.MethodGet, "/playground/runtime.wasm?v="+testReleaseVersion, nil)
	conditionalRequest.Header.Set("Accept-Encoding", "gzip")
	conditionalRequest.Header.Set("If-None-Match", etag)
	conditionalResponse := httptest.NewRecorder()
	handler.ServeHTTP(conditionalResponse, conditionalRequest)
	if conditionalResponse.Code != http.StatusNotModified {
		t.Fatalf("conditional status = %d, want %d", conditionalResponse.Code, http.StatusNotModified)
	}
	if conditionalResponse.Body.Len() != 0 {
		t.Fatalf("conditional body = %d bytes", conditionalResponse.Body.Len())
	}

	identityRequest := httptest.NewRequest(http.MethodGet, "/playground/runtime.wasm?v="+testReleaseVersion, nil)
	identityResponse := httptest.NewRecorder()
	handler.ServeHTTP(identityResponse, identityRequest)
	if got := identityResponse.Header().Get("ETag"); got == "" || got == etag {
		t.Fatalf("identity ETag = %q, gzip ETag = %q", got, etag)
	}

	for _, url := range []string{
		"/playground/runtime.wasm",
		"/playground/runtime.wasm?v=v0.33.1",
	} {
		request := httptest.NewRequest(http.MethodHead, url, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if got := response.Header().Get("Cache-Control"); got != "public, max-age=0, must-revalidate" {
			t.Fatalf("%s Cache-Control = %q", url, got)
		}
	}
}

func TestPlaygroundRuntimeHandlerRejectsUnsupportedMethod(t *testing.T) {
	handler := playgroundRuntimeHandler(t.TempDir(), testReleaseVersion)
	request := httptest.NewRequest(http.MethodPost, "/playground/runtime.wasm?v="+testReleaseVersion, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if got := response.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q", got)
	}
}

func writeRuntimeFixture(t *testing.T, root string, raw []byte, withRaw, withGzip bool) []byte {
	t.Helper()
	dir := filepath.Join(root, "public", "playground")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if withRaw {
		if err := os.WriteFile(filepath.Join(dir, "runtime.wasm"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !withGzip {
		return nil
	}

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), compressed.Bytes()...)
	if err := os.WriteFile(filepath.Join(dir, "runtime.wasm.gz"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return data
}

func headerValuesContain(values []string, want string) bool {
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(item), want) {
				return true
			}
		}
	}
	return false
}
