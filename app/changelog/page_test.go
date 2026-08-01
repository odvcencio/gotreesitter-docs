package changelog

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/odvcencio/gotreesitter-docs/internal/releasecatalog"
	"m31labs.dev/gosx"
	"m31labs.dev/gosx/route"
)

func TestLoadChangelogReturnsCatalogError(t *testing.T) {
	originalErr := catalogErr
	catalogErr = errors.New("snapshot mismatch")
	t.Cleanup(func() {
		catalogErr = originalErr
	})

	ctx := &route.RouteContext{
		Request: httptest.NewRequest("GET", "/changelog", nil),
	}
	_, err := loadChangelog(ctx, route.FilePage{})
	if err == nil || !strings.Contains(err.Error(), "snapshot mismatch") {
		t.Fatalf("error = %v, want snapshot mismatch", err)
	}
}

func TestLoadChangelogSeparatesReleasedRecoveryFix(t *testing.T) {
	ctx := &route.RouteContext{
		Request: httptest.NewRequest(
			"GET",
			"/changelog?q=deferred+parent+links&status=released",
			nil,
		),
	}
	loaded, err := loadChangelog(ctx, route.FilePage{})
	if err != nil {
		t.Fatal(err)
	}
	data := loaded.(map[string]any)
	releases := data["releases"].([]map[string]any)
	if len(releases) == 0 || releases[0]["version"] != "v0.47.1" {
		t.Fatalf("first result = %#v, want v0.47.1", releases)
	}
	for _, release := range releases {
		if release["status"] != string(releasecatalog.StatusReleased) {
			t.Fatalf("released filter returned %#v", release)
		}
	}
}

func TestLoadChangelogShowsRetirementCampaignInV048(t *testing.T) {
	ctx := &route.RouteContext{
		Request: httptest.NewRequest(
			"GET",
			"/changelog?q=expected+Hurl+and+INI+root+types&category=Fixed&status=released",
			nil,
		),
	}
	loaded, err := loadChangelog(ctx, route.FilePage{})
	if err != nil {
		t.Fatal(err)
	}
	data := loaded.(map[string]any)
	releases := data["releases"].([]map[string]any)
	if len(releases) != 1 || releases[0]["version"] != "v0.48.0" {
		t.Fatalf("results = %#v, want v0.48.0 only", releases)
	}
	if releases[0]["statusLabel"] != "Released · immutable" {
		t.Fatalf("status label = %q", releases[0]["statusLabel"])
	}
}

func TestLoadChangelogDoesNotInventUnreleasedEntries(t *testing.T) {
	ctx := &route.RouteContext{
		Request: httptest.NewRequest("GET", "/changelog?status=unreleased", nil),
	}
	loaded, err := loadChangelog(ctx, route.FilePage{})
	if err != nil {
		t.Fatal(err)
	}
	data := loaded.(map[string]any)
	if data["hasResults"] != false {
		t.Fatalf("hasResults = %#v, want false", data["hasResults"])
	}
	if releases := data["releases"].([]map[string]any); len(releases) != 0 {
		t.Fatalf("unreleased results = %#v, want none", releases)
	}
}

func TestFilterFormUsesManagedGETAndPreservesSelection(t *testing.T) {
	node := renderFilterForm(
		"recovery",
		"Fixed",
		releasecatalog.StatusReleased,
		[]string{"Added", "Fixed"},
	)
	html := gosx.RenderHTML(node)
	for _, required := range []string{
		`method="get"`,
		`action="/changelog"`,
		`data-gosx-form`,
		`data-gosx-form-mode="get"`,
		`value="recovery"`,
		`value="Fixed" selected`,
		`value="released" selected`,
	} {
		if !strings.Contains(html, required) {
			t.Errorf("managed filter form does not contain %q:\n%s", required, html)
		}
	}
}

func TestReleaseEvidenceLinksStayPinned(t *testing.T) {
	release := catalog.Releases[1]
	if got := releaseEvidenceURL(release); got != repositoryURL+"/releases/tag/v0.48.0" {
		t.Fatalf("release evidence URL = %q", got)
	}
	if got := releaseCodeURL(release, 1); got != repositoryURL+"/compare/v0.47.1...v0.48.0" {
		t.Fatalf("release code URL = %q", got)
	}
	if got := sourceLineURL(release.SourceLine); !strings.HasPrefix(got, releasecatalog.SourceURL+"#L") {
		t.Fatalf("source URL = %q", got)
	}
	if title, body := releaseNarrative(release); title == "" || body == "" {
		t.Fatalf("v0.48.0 narrative = (%q, %q), want content", title, body)
	}
	trail := historicalTrail(catalog.Releases[2])
	if len(trail) != 3 || trail[0]["href"] != repositoryURL+"/issues/490" {
		t.Fatalf("v0.47.1 trail = %#v", trail)
	}
}

func TestExtractReferencesLinksNumericCommit(t *testing.T) {
	references := extractReferences("Fixed by `1234567`.")
	if len(references) != 1 {
		t.Fatalf("references = %#v, want one commit", references)
	}
	if references[0]["href"] != repositoryURL+"/commit/1234567" {
		t.Fatalf("commit reference = %#v", references[0])
	}
}

func TestChangelogMetadataUsesInspectedSocialCard(t *testing.T) {
	meta := changelogMetadata()
	if meta.OpenGraph == nil || len(meta.OpenGraph.Images) != 1 {
		t.Fatalf("Open Graph metadata = %#v", meta.OpenGraph)
	}
	image := meta.OpenGraph.Images[0]
	if image.Width != 1200 || image.Height != 630 ||
		!strings.Contains(image.URL, "/social/changelog.png") {
		t.Fatalf("Open Graph image = %#v", image)
	}
	if meta.Twitter == nil || meta.Twitter.Card != "summary_large_image" {
		t.Fatalf("Twitter metadata = %#v", meta.Twitter)
	}
}

func TestChangelogPageUsesNativeAccessibleControls(t *testing.T) {
	source, err := pageSource.ReadFile("page.gsx")
	if err != nil {
		t.Fatal(err)
	}
	page := string(source)
	for _, required := range []string{
		`<details`,
		`<summary`,
		`aria-live="polite"`,
		`aria-label="Adjacent versions"`,
		`release.status`,
		`entry.sourceURL`,
	} {
		if !strings.Contains(page, required) {
			t.Errorf("page.gsx does not contain %q", required)
		}
	}
	for _, forbidden := range []string{"<script", "onclick=", "onchange="} {
		if strings.Contains(strings.ToLower(page), forbidden) {
			t.Errorf("page.gsx contains client escape hatch %q", forbidden)
		}
	}
}
