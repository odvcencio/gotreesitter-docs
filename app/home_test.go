package docs

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var literalVersion = regexp.MustCompile(`v0\.[0-9]+\.[0-9]+`)

func TestHomeShellDoesNotHardCodeTheGotreesitterVersion(t *testing.T) {
	for _, name := range []string{"page.gsx", "page.server.go", "layout.gsx"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if loc := literalVersion.FindIndex(data); loc != nil {
			t.Errorf("%s hard-codes %q; bind gtsVersion instead", name, data[loc[0]:loc[1]])
		}
	}
}

func TestHomePlaygroundLinksCarryTheAddSample(t *testing.T) {
	data, err := os.ReadFile("page.gsx")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "sample=add"); got != 6 {
		t.Fatalf("page.gsx has %d sample=add links, want 6", got)
	}
}
