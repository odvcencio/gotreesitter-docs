package docs

import (
	"os"
	"regexp"
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
