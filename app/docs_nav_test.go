package docs

import (
	"strings"
	"testing"
)

func TestDocsFrontmatterHasTitleDescriptionAndKnownNavigationGroup(t *testing.T) {
	documents := docsLibrary.Collection("docs")
	if len(documents) == 0 {
		t.Fatal("content/docs contains no parsed documents")
	}

	for _, doc := range documents {
		t.Run(doc.Path, func(t *testing.T) {
			if doc.Parsed == nil {
				t.Fatal("document frontmatter was not parsed")
			}
			if title := strings.TrimSpace(doc.Frontmatter["title"]); title == "" {
				t.Error("frontmatter title is required")
			}
			if description := strings.TrimSpace(doc.Frontmatter["description"]); description == "" {
				t.Error("frontmatter description is required")
			}
			if group := strings.TrimSpace(doc.Frontmatter["nav_group"]); !docsNavGroupKnown(group) {
				t.Errorf("frontmatter nav_group %q is not one of docsNavGroupOrder", group)
			}
		})
	}
}
