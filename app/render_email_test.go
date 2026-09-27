package docs

import (
	"regexp"
	"strings"
	"testing"

	"m31labs.dev/gosx"
)

var emailShape = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+`)

func TestRenderedContentProtectsEmailAddresses(t *testing.T) {
	for _, doc := range docsLibrary.Collection("docs") {
		node, err := RenderDesignDoc(doc)
		if err != nil {
			t.Fatalf("render %s: %v", doc.Slug, err)
		}
		assertNoUnprotectedEmail(t, doc.Slug, gosx.RenderHTML(node))
	}

	node, err := RenderMarkdownFragment("contact test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	assertNoUnprotectedEmail(t, "markdown fragment", gosx.RenderHTML(node))
}

func assertNoUnprotectedEmail(t *testing.T, name, html string) {
	t.Helper()
	outside := strings.Builder{}
	inside := false
	for len(html) > 0 {
		if !inside {
			at := strings.Index(html, "<!--email_off-->")
			if at < 0 {
				outside.WriteString(html)
				break
			}
			outside.WriteString(html[:at])
			html = html[at+len("<!--email_off-->"):]
			inside = true
			continue
		}
		at := strings.Index(html, "<!--/email_off-->")
		if at < 0 {
			t.Fatalf("%s has an unclosed email_off marker", name)
		}
		html = html[at+len("<!--/email_off-->"):]
		inside = false
	}
	if match := emailShape.FindString(outside.String()); match != "" {
		t.Errorf("%s has email-shaped text outside email_off markers: %s", name, match)
	}
}
