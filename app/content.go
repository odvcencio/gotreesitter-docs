package docs

import (
	"log"
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
	root := server.ResolveAppRoot(thisFile)

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
