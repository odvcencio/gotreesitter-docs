package docs

import (
	"runtime/debug"
	"testing"
)

func TestResolvePlaygroundGTSVersion(t *testing.T) {
	tests := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{name: "missing build info", want: "dev"},
		{name: "release dependency", info: &debug.BuildInfo{Deps: []*debug.Module{{Path: "github.com/odvcencio/gotreesitter", Version: "v0.36.0"}}}, want: "v0.36.0"},
		{name: "versioned replacement", info: &debug.BuildInfo{Deps: []*debug.Module{{Path: "github.com/odvcencio/gotreesitter", Version: "v0.36.0", Replace: &debug.Module{Path: "github.com/example/fork", Version: "v0.36.1"}}}}, want: "v0.36.1"},
		{name: "local replacement", info: &debug.BuildInfo{Deps: []*debug.Module{{Path: "github.com/odvcencio/gotreesitter", Version: "v0.36.0", Replace: &debug.Module{Path: "../gotreesitter"}}}}, want: "dev"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolvePlaygroundGTSVersion(test.info); got != test.want {
				t.Fatalf("version = %q, want %q", got, test.want)
			}
		})
	}
}
