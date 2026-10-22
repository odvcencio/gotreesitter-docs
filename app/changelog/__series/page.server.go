package series

import (
	"embed"
	"strings"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	"github.com/odvcencio/gotreesitter-docs/app/changelog"
	"m31labs.dev/gosx/route"
	"m31labs.dev/gosx/server"
)

//go:embed page.gsx
var pageSource embed.FS

// /changelog/{series} renders the releases of one minor version (v0.55) or the
// unreleased section. The index at /changelog links here.
func init() {
	docsapp.RegisterDocsPage(
		"Changelog",
		"GoTreeSitter releases of one minor version, with entries, evidence links and source lines.",
		route.FileModuleOptions{
			Load: changelog.LoadRelease,
			Metadata: func(ctx *route.RouteContext, page route.FilePage, data any) (server.Metadata, error) {
				values, _ := data.(map[string]any)
				label, _ := values["label"].(string)
				series, _ := values["series"].(string)
				meta := docsapp.CanonicalMetadata("/changelog/" + series)
				meta.Title = server.Title{Absolute: strings.TrimSpace(label) + " releases | gotreesitter"}
				meta.Description = "GoTreeSitter " + label + " releases with entries, evidence links and source lines."
				return meta, nil
			},
		},
	)
}
