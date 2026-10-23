package docs

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func pinnedGTSVersion(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\s*github\.com/odvcencio/gotreesitter (v[0-9]+\.[0-9]+\.[0-9]+)`).FindStringSubmatch(string(data))
	if m == nil {
		t.Fatal("go.mod does not pin gotreesitter")
	}
	return m[1]
}

// Install commands and tagged source links must follow go.mod, except on the
// history pages that describe a specific older release.
func TestDocsInstallCommandsAndSourceLinksFollowThePinnedVersion(t *testing.T) {
	pinned := pinnedGTSVersion(t)
	stalePin := regexp.MustCompile(`gotreesitter@v[0-9]+\.[0-9]+\.[0-9]+`)
	staleLink := regexp.MustCompile(`/(?:blob|tree)/v[0-9]+\.[0-9]+\.[0-9]+/`)
	files, _ := filepath.Glob(filepath.Join("..", "content", "docs", "*.md"))
	for _, file := range files {
		if strings.HasPrefix(filepath.Base(file), "release-") {
			continue
		}
		data, _ := os.ReadFile(file)
		for _, m := range stalePin.FindAllString(string(data), -1) {
			if m != "gotreesitter@"+pinned {
				t.Errorf("%s pins %s; go.mod pins %s", file, m, pinned)
			}
		}
		for _, m := range staleLink.FindAllString(string(data), -1) {
			if !strings.Contains(m, pinned) {
				t.Errorf("%s links tagged source %s; retarget to %s or justify it in prose", file, m, pinned)
			}
		}
	}
}

// Claims the 2026-09-27 audit retired must not come back.
func TestDocsAvoidOverstatedClaims(t *testing.T) {
	banned := []string{"byte-exact", "byte-for-byte", "0 lines of C", "zero lines of C", "holds every one", "1.5x C", "439.825", "27.4 ns"}
	files, _ := filepath.Glob(filepath.Join("..", "content", "docs", "*.md"))
	for _, file := range files {
		data, _ := os.ReadFile(file)
		for _, phrase := range banned {
			if strings.Contains(strings.ToLower(string(data)), strings.ToLower(phrase)) {
				t.Errorf("%s contains the retired claim %q", file, phrase)
			}
		}
	}
}
