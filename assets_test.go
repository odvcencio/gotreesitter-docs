package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

func TestPlaygroundWASMTransportAndCache(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "public", "playground", "runtime.wasm")
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte("\x00asm\x01\x00\x00\x00")
	if err := os.WriteFile(name, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	writer := brotli.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name+".br", compressed.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	version := hex.EncodeToString(sum[:6])
	handler := playgroundWASMCompression(root)(http.NotFoundHandler())
	for _, tc := range []struct {
		encoding, method, version string
		compressed, immutable     bool
	}{
		{"gzip, br", "GET", version, true, true},
		{"br;q=0", "GET", version, false, true},
		{"", "GET", "", false, false},
		{"br", "GET", "stale", true, false},
		{"br", "HEAD", version, true, true},
	} {
		t.Run(tc.encoding+tc.method+tc.version, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "/playground/runtime.wasm?v="+tc.version, nil)
			r.Header.Set("Accept-Encoding", tc.encoding)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 200 || w.Header().Get("Content-Type") != "application/wasm" {
				t.Fatalf("response: %d %v", w.Code, w.Header())
			}
			if strings.Contains(w.Header().Get("Cache-Control"), "immutable") != tc.immutable {
				t.Fatal(w.Header())
			}
			if w.Header().Get("Vary") != "Accept-Encoding" {
				t.Fatal(w.Header())
			}
			if (w.Header().Get("Content-Encoding") == "br") != tc.compressed {
				t.Fatal(w.Header())
			}
			if tc.method == "HEAD" {
				if w.Body.Len() != 0 {
					t.Fatal("HEAD returned a body")
				}
				return
			}
			body := w.Body.Bytes()
			if tc.compressed {
				var err error
				body, err = io.ReadAll(brotli.NewReader(bytes.NewReader(body)))
				if err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(body, data) {
				t.Fatal("transport changed WASM bytes")
			}
		})
	}
	if err := os.Remove(name + ".br"); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/playground/runtime.wasm", nil)
	r.Header.Set("Accept-Encoding", "br")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Header().Get("Content-Encoding") != "" || !bytes.Equal(w.Body.Bytes(), data) {
		t.Fatal("missing sidecar did not fall back")
	}
}
