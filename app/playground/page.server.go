// Package playground backs the live /playground route (page.gsx): the real
// gotreesitter engine compiled to WASM, parsing in the visitor's tab. This
// file only stages data for the template — the Go-WASM client lives in
// app/playground/client and the bounded detection API lives in
// app/playground_api.go.
package playground

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	"m31labs.dev/gosx/route"
	"m31labs.dev/gosx/server"
)

// initialGoSource is the buffer the page ships server-rendered inside the
// <textarea>, so the first paint already shows real code while the runtime
// downloads. The Go-WASM engine reads this back as its "go" try-sample.
const initialGoSource = `package main

import "fmt"

func greet(name string) string {
	return "hey, " + name
}

func main() {
	fmt.Println(greet("gopher"))
}
`

// playgroundAppRoot resolves the repo root the same way app/content.go does:
// this package lives two directories below it.
func playgroundAppRoot() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return server.ResolveAppRoot(filepath.Join(filepath.Dir(thisFile), "..", "..", "main.go"))
}

func init() {
	root := playgroundAppRoot()
	docsapp.RegisterStaticDocsPage(
		"Playground",
		"The actual production parser, compiled to WASM, running in your tab. Pick a language or just start typing — it'll figure it out.",
		route.FileModuleOptions{
			Load: func(ctx *route.RouteContext, page route.FilePage) (any, error) {
				version := docsapp.PlaygroundGTSVersion()
				// Stat per request so `scripts/build-playground-wasm.sh` takes
				// effect on reload, without restarting the server.
				transferBytes := int64(0)
				wasmReady := false
				if info, err := os.Stat(filepath.Join(root, "public", "playground", "runtime.wasm")); err == nil && !info.IsDir() {
					transferBytes = info.Size()
					wasmReady = true
				}
				if info, err := os.Stat(filepath.Join(root, "public", "playground", "runtime.wasm.gz")); err == nil && !info.IsDir() {
					transferBytes = info.Size()
				}
				return map[string]any{
					"wasmReady": wasmReady,
					"wasmMB":    fmt.Sprintf("%.1f MB", float64(transferBytes)/1_000_000),
					// Exact release keys receive immutable caching; unversioned
					// or stale keys revalidate so release bumps cannot reuse an
					// older runtime.
					"wasmURL":           "/playground/runtime.wasm?v=" + version,
					"gtsVersion":        version,
					"languagesURL":      "/playground/langs.json",
					"languageURLPrefix": "/playground/lang/",
					"detectURL":         "/playground/detect",
					"initialSource":     initialGoSource,
				}, nil
			},
			Metadata: func(ctx *route.RouteContext, page route.FilePage, data any) (server.Metadata, error) {
				return server.Metadata{
					Title: server.Title{Absolute: "Playground — gotreesitter"},
					Links: []server.LinkTag{{
						Rel:  "stylesheet",
						Href: docsapp.PublicAssetURL("playground/playground.css"),
					}},
				}, nil
			},
		},
	)
}
