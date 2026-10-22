package docs

import (
	"strings"
	"testing"

	"m31labs.dev/gosx"
)

func TestFencedCodeBlocksCarryGoSXCopyControls(t *testing.T) {
	node, err := RenderMarkdownFragment("```go title=main.go\npackage main\n```\n")
	if err != nil {
		t.Fatal(err)
	}
	html := gosx.RenderHTML(node)
	for _, want := range []string{
		`class="code" data-gosx-copy-scope=""`,
		`<pre class="codebody"><code>`,
		`data-gosx-copy-button=""`,
		`data-gosx-copy-label="Copy"`,
		`data-gosx-copy-status=""`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered code block lacks %q:\n%s", want, html)
		}
	}
}
