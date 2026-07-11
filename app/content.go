package docs

import (
	"log"
	"path/filepath"
	"runtime"

	"m31labs.dev/gosx/content"
	"m31labs.dev/gosx/server"
)

// docsLibrary is loaded once at package-variable initialization time, which
// the Go spec guarantees runs before any init() in this package (including
// the route/module registrations in layout.server.go and the [...slug]
// catch-all's registration), so every reader sees a populated library.
var docsLibrary = mustLoadDocsLibrary()

func mustLoadDocsLibrary() content.Library {
	_, thisFile, _, _ := runtime.Caller(0)
	// This package lives one directory below the application root. Give
	// ResolveAppRoot a synthetic root-level caller so package tests do not
	// resolve `app/` itself as the fallback root before main initializes.
	root := server.ResolveAppRoot(filepath.Join(filepath.Dir(thisFile), "..", "main.go"))

	library, err := content.LoadWithOptions(root, content.LoadOptions{
		RenderOptions: content.MDPPOptions{
			HighlightCode: true,
			HeadingIDs:    true,
		},
	}, content.Collection{Name: "docs", Dir: "content/docs"})
	if err != nil {
		log.Fatalf("gotreesitter-docs: load content/docs collection: %v", err)
	}
	return library
}

// DocsLibrary returns the loaded content library backing the /docs/*
// catch-all route. Exported so sibling file-route packages (e.g.
// app/docs/__slug) can resolve documents by slug without reloading the
// collection.
func DocsLibrary() content.Library {
	return docsLibrary
}
