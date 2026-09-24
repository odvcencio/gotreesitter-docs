package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"m31labs.dev/gosx/server"
)

// playgroundWASMCompression serves the build's Brotli transport variant.
// The URL version remains the hash of the decoded WASM bytes.
func playgroundWASMCompression(root string) server.Middleware {
	name := filepath.Join(root, "public", "playground", "runtime.wasm")
	var version string
	if data, err := os.ReadFile(name); err == nil {
		sum := sha256.Sum256(data)
		version = hex.EncodeToString(sum[:6])
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/playground/runtime.wasm" || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Add("Vary", "Accept-Encoding")
			w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
			if version != "" && r.URL.Query().Get("v") == version {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			if acceptsBrotli(r.Header.Get("Accept-Encoding")) {
				if info, err := os.Stat(name + ".br"); err == nil && !info.IsDir() {
					w.Header().Set("Content-Type", "application/wasm")
					w.Header().Set("Content-Encoding", "br")
					http.ServeFile(w, r, name+".br")
					return
				}
			}
			// Serve the decoded variant with the same cache policy.
			w.Header().Set("Content-Type", "application/wasm")
			http.ServeFile(w, r, name)
		})
	}
}

func acceptsBrotli(value string) bool {
	for _, item := range strings.Split(value, ",") {
		parts := strings.Split(item, ";")
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "br") {
			continue
		}
		for _, parameter := range parts[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if ok && strings.EqualFold(key, "q") {
				quality, err := strconv.ParseFloat(value, 64)
				if err != nil || quality <= 0 || quality > 1 {
					return false
				}
			}
		}
		return true
	}
	return false
}
