package docs

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/odvcencio/gotreesitter-docs/internal/releasecatalog"
)

func TestHomepageFactsMatchPinnedRelease(t *testing.T) {
	page, err := os.ReadFile("page.gsx")
	if err != nil {
		t.Fatal(err)
	}
	facts := regexp.MustCompile(`(?s)<p class="home-facts">(.*?)</p>`).FindSubmatch(page)
	if len(facts) != 2 {
		t.Fatal("homepage release facts are missing")
	}
	catalog, err := releasecatalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	date := ""
	for _, release := range catalog.Releases {
		if release.Tag == releasecatalog.LatestReleasedVersion {
			date = release.Date
		}
	}
	if date == "" {
		t.Fatal("latest released version has no catalog date")
	}
	wants := []string{"Released " + date}
	counts := regexp.MustCompile(`(\d+) languages, with (\d+) Go external scanners and (\d+) default token sources`)
	for _, feature := range landingFeatures {
		if match := counts.FindStringSubmatch(fmt.Sprint(feature["body"])); len(match) == 4 {
			wants = append(wants, match[1]+" grammars", match[2]+" Go external scanners", match[3]+" default token sources")
		}
	}
	if len(wants) != 4 {
		t.Fatal("registry feature counts are missing")
	}
	for _, want := range wants {
		if !strings.Contains(string(facts[1]), want) {
			t.Errorf("homepage facts do not contain %q", want)
		}
	}
}
