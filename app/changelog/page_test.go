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

func TestChangelogReleaseReportUsesPinnedUpstreamLink(t *testing.T) {
	ctx := &route.RouteContext{Request: httptest.NewRequest("GET", "/changelog?q=dated+exception", nil)}
	loaded, err := loadChangelog(ctx, route.FilePage{})
	if err != nil {
		t.Fatal(err)
	}
	want := repositoryURL + "/blob/" + releasecatalog.SourceCommit + "/docs/releasing.md#v0520-only-tag-creation-exception"
	found := false
	for _, release := range loaded.(map[string]any)["releases"].([]map[string]any) {
		for _, section := range release["sections"].([]map[string]any) {
			for _, entry := range section["entries"].([]map[string]any) {
				html := gosx.RenderHTML(entry["content"].(gosx.Node))
				if strings.Contains(html, `href="docs/`) {
					t.Fatalf("unresolved source link: %s", html)
				}
				found = found || strings.Contains(html, `href="`+want+`"`)
			}
		}
	}
	if !found {
		t.Fatalf("missing pinned report link %s", want)
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

func TestBuildVersionLinksSkipsEmptyReleases(t *testing.T) {
	links := buildVersionLinks()
	for _, link := range links {
		if link["version"] == "Unreleased" {
			t.Fatalf("version links contain empty Unreleased: %#v", links)
		}
	}
	// Derive the expected anchor from the catalog rather than pinning a
	// version, so cutting a release does not fail this test for the wrong
	// reason. The invariant under test is ordering: newest released first.
	wantHref := "#release-" + strings.NewReplacer(".", "-").Replace(releasecatalog.LatestReleasedVersion)
	if len(links) == 0 || links[0]["href"] != wantHref {
		t.Fatalf("first version link = %#v, want %s", links, wantHref)
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

// releaseByTag finds a release in the loaded catalog. Tests that assert on a
// specific release's content look it up by tag instead of by slice index, which
// shifts by one on every release and fails for reasons unrelated to the code.
func releaseByTag(t *testing.T, tag string) releasecatalog.Release {
	t.Helper()
	for _, release := range catalog.Releases {
		if release.Tag == tag {
			return release
		}
	}
	t.Fatalf("release %s is not in the catalog", tag)
	return releasecatalog.Release{}
}

func TestReleaseEvidenceLinksStayPinned(t *testing.T) {
	// The newest released entry, whatever it currently is.
	release := releaseByTag(t, releasecatalog.LatestReleasedVersion)
	wantEvidence := repositoryURL + "/releases/tag/" + releasecatalog.LatestReleasedVersion
	if got := releaseEvidenceURL(release); got != wantEvidence {
		t.Fatalf("release evidence URL = %q, want %q", got, wantEvidence)
	}
	// releaseCodeURL compares against the release one position older, so the
	// link must name two different versions and end at the newest one.
	code := releaseCodeURL(release, 1)
	if !strings.HasPrefix(code, repositoryURL+"/compare/") ||
		!strings.HasSuffix(code, "..."+releasecatalog.LatestReleasedVersion) {
		t.Fatalf("release code URL = %q", code)
	}
	if got := sourceLineURL(release.SourceLine); !strings.HasPrefix(got, releasecatalog.SourceURL+"#L") {
		t.Fatalf("source URL = %q", got)
	}
	if title, body := releaseNarrative(release); title == "" || body == "" {
		t.Fatalf("%s narrative = (%q, %q), want content", releasecatalog.LatestReleasedVersion, title, body)
	}
	// v0.47.1 is named directly because the assertion is about that release's
	// own issue trail, not about whichever release happens to be latest.
	trail := historicalTrail(releaseByTag(t, "v0.47.1"))
	if len(trail) != 3 || trail[0]["href"] != repositoryURL+"/issues/490" {
		t.Fatalf("v0.47.1 trail = %#v", trail)
	}
}

func TestExtractReferencesLinksVerifiedCommit(t *testing.T) {
	var known string
	for _, ref := range strings.Fields(knownCommitSource) {
		known = ref
		break
	}
	if known == "" {
		t.Fatal("verified commit allowlist is empty")
	}
	references := extractReferences("Fixed by `" + known + "`.")
	if len(references) != 1 {
		t.Fatalf("references = %#v, want one commit", references)
	}
	if references[0]["href"] != repositoryURL+"/commit/"+known {
		t.Fatalf("commit reference = %#v", references[0])
	}
}

func TestExtractReferencesSkipsBrokenChangelogTargets(t *testing.T) {
	broken := []string{
		"172ada1cc4117d0260d9340680b4134adba2bc2c",
		"41d6e5fe811ec94229ee71771174a8cce558dfee",
		"48ab75f29abaa315fad7fa7b8338f92bb07376a7",
		"5739fd79bcfc75ba7526773d0cf634521f8aca3c",
		"587f30d184b058450be2a2330878210c5f33b3f9",
		"61a7c75e225e3035390be32d635545e40d8c5faf",
	}
	for _, sha := range broken {
		t.Run(sha[:8], func(t *testing.T) {
			if got := extractReferences("Grammar source `" + sha + "`."); len(got) != 0 {
				t.Fatalf("references = %#v, want no link for an unknown commit", got)
			}
		})
	}
	if got := extractReferences("CI run `32609724840`."); len(got) != 1 ||
		got[0]["href"] != repositoryURL+"/actions/runs/32609724840" {
		t.Fatalf("resolved action run = %#v", got)
	}
	if got := extractReferences("CI run `32609724841`."); len(got) != 0 {
		t.Fatalf("unknown action run = %#v, want no link", got)
	}
	if got := extractReferences("The `v0.52.0-only` tag exception."); len(got) != 0 {
		t.Fatalf("suffixed version reference = %#v, want no tag link", got)
	}
	if got := extractReferences("See v0.54.0."); len(got) != 1 ||
		got[0]["href"] != repositoryURL+"/releases/tag/v0.54.0" {
		t.Fatalf("real release tag = %#v", got)
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

func TestArchivedReleaseViewsLinkToArchiveLines(t *testing.T) {
	release := releaseByTag(t, "v0.48.0")
	views, _, err := buildReleaseViews([]releasecatalog.Release{release})
	if err != nil {
		t.Fatal(err)
	}
	prefix := repositoryURL + "/blob/" + releasecatalog.SourceCommit + "/docs/changelog/archive-1.md#L"
	if !strings.HasPrefix(views[0]["sourceURL"].(string), prefix) {
		t.Fatalf("release source URL = %v", views[0]["sourceURL"])
	}
	for _, section := range views[0]["sections"].([]map[string]any) {
		if !strings.HasPrefix(section["sourceURL"].(string), prefix) {
			t.Fatalf("section source URL = %v", section["sourceURL"])
		}
		for _, entry := range section["entries"].([]map[string]any) {
			if !strings.HasPrefix(entry["sourceURL"].(string), prefix) {
				t.Fatalf("entry source URL = %v", entry["sourceURL"])
			}
		}
	}
}

func TestReleasePerformanceTableIsRendered(t *testing.T) {
	views, _, err := buildReleaseViews([]releasecatalog.Release{releaseByTag(t, "v0.54.0")})
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range views[0]["sections"].([]map[string]any) {
		if section["name"] != "Performance evidence" {
			continue
		}
		if section["hasIntroduction"] != true {
			t.Fatal("release table is hidden")
		}
		html := gosx.RenderHTML(section["introduction"].(gosx.Node))
		if !strings.Contains(html, "5.42x") || !strings.Contains(html, "2.87x") || !strings.Contains(html, `class="tbl"`) {
			t.Fatalf("missing performance table: %s", html)
		}
		return
	}
	t.Fatal("missing performance section")
}
