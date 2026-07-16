package docs

import (
	"runtime/debug"
	"sync"
)

var (
	playgroundVersionOnce sync.Once
	playgroundVersionVal  string
)

// PlaygroundGTSVersion reports the gotreesitter module version linked into
// the site. It is displayed in the shared shell and playground result page.
func PlaygroundGTSVersion() string {
	playgroundVersionOnce.Do(func() {
		info, ok := debug.ReadBuildInfo()
		if !ok {
			playgroundVersionVal = "dev"
			return
		}
		playgroundVersionVal = resolvePlaygroundGTSVersion(info)
	})
	return playgroundVersionVal
}

func resolvePlaygroundGTSVersion(info *debug.BuildInfo) string {
	if info == nil {
		return "dev"
	}
	for _, dep := range info.Deps {
		if dep == nil || dep.Path != "github.com/odvcencio/gotreesitter" {
			continue
		}
		if dep.Replace != nil {
			if dep.Replace.Version == "" || dep.Replace.Version == "(devel)" {
				return "dev"
			}
			return dep.Replace.Version
		}
		if dep.Version == "" || dep.Version == "(devel)" {
			return "dev"
		}
		return dep.Version
	}
	return "dev"
}
