package docs

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"sync"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/ir"
	islandprogram "m31labs.dev/gosx/island/program"
	"m31labs.dev/gosx/server"
)

const (
	langSearchIslandName  = "LangSearch"
	LangSearchProgramPath = "/gosx/islands/LangSearch.json"
)

//go:embed lang_search.gsx
var langSearchSource embed.FS

var (
	langSearchProgramOnce sync.Once
	langSearchProgramVal  *islandprogram.Program
	langSearchProgramErr  error
	langSearchVersionOnce sync.Once
	langSearchVersionVal  string
)

// LangSearchProgram compiles the checked-in .gsx island through GoSX's normal
// Compile -> LowerIsland pipeline. There is no hand-authored opcode program.
func LangSearchProgram() *islandprogram.Program {
	langSearchProgramOnce.Do(func() {
		source, err := langSearchSource.ReadFile("lang_search.gsx")
		if err != nil {
			langSearchProgramErr = err
			return
		}
		compiled, err := gosx.Compile(source)
		if err != nil {
			langSearchProgramErr = err
			return
		}
		for i, component := range compiled.Components {
			if component.Name != langSearchIslandName || !component.IsIsland {
				continue
			}
			langSearchProgramVal, langSearchProgramErr = ir.LowerIsland(compiled, i)
			return
		}
		langSearchProgramErr = fmt.Errorf("GoSX island %q not found", langSearchIslandName)
	})
	if langSearchProgramErr != nil {
		panic(langSearchProgramErr)
	}
	return langSearchProgramVal
}

func LangSearchProgramContentVersion() string {
	langSearchVersionOnce.Do(func() {
		data, err := islandprogram.EncodeJSON(LangSearchProgram())
		if err != nil {
			return
		}
		sum := sha256.Sum256(data)
		langSearchVersionVal = hex.EncodeToString(sum[:6])
	})
	return langSearchVersionVal
}

func LangSearchProgramURL() string {
	version := LangSearchProgramContentVersion()
	if version == "" {
		return LangSearchProgramPath
	}
	return LangSearchProgramPath + "?v=" + version
}

func langSearchProps(names []string) map[string]any {
	langs := make([]map[string]any, 0, len(names))
	for i, name := range names {
		langs = append(langs, map[string]any{
			"name":  name,
			"color": dotPalette[i%len(dotPalette)],
			"ts":    langTokenSourceSet[name],
		})
	}
	return map[string]any{"langs": langs}
}

func BuildLangGridIsland(rt *server.PageRuntime, names []string) gosx.Node {
	if rt == nil {
		return gosx.Text("")
	}
	rt.SetProgramAsset(langSearchIslandName, LangSearchProgramURL(), "json", LangSearchProgramContentVersion())
	return rt.Island(LangSearchProgram(), langSearchProps(names))
}
