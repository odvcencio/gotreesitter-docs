package docs

import (
	"log"

	"m31labs.dev/gosx/route"
)

// init registers a file module for the root layout (app/layout.gsx) so its
// Bindings run on every request, regardless of which page rendered. That
// makes navGroups available to DocsNavigation() in layout.gsx without every
// page.server.go having to thread it through its own Load.
func init() {
	if err := route.RegisterFileModuleHere(route.FileModuleOptions{
		Bindings: func(ctx *route.RouteContext, page route.FilePage, data any) route.FileTemplateBindings {
			return route.FileTemplateBindings{
				Values: map[string]any{
					"navGroups": buildDocsNavGroups(),
				},
			}
		},
	}); err != nil {
		log.Fatal(err)
	}
}
