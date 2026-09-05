package docs

import (
	"strings"
	"testing"

	"m31labs.dev/gosx"
)

func TestRenderMarkdownFragmentSourceLinks(t *testing.T) {
	const base = "https://github.com/odvcencio/gotreesitter/blob/2295871057f860598a006d6068588a6303fefb02/"
	for _, tc := range []struct{ name, source, base, want string }{
		{"relative", "[report](docs/performance/report.md)", base, base + "docs/performance/report.md"},
		{"absolute", "[issue](https://example.com/issue)", base, "https://example.com/issue"},
		{"default", "[local](/docs/performance)", "", "/docs/performance"},
		{"nested", "- **[report](./docs/report.md#results)**", base, base + "docs/report.md#results"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node, err := RenderMarkdownFragmentWithBase(tc.source, tc.base)
			if err != nil {
				t.Fatal(err)
			}
			html := gosx.RenderHTML(node)
			if !strings.Contains(html, `href="`+tc.want+`"`) {
				t.Fatalf("rendered %s, want destination %s", html, tc.want)
			}
		})
	}
}
