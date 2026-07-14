//go:build js && wasm

package main

import (
	"fmt"
	"strconv"
	"sync"
	"syscall/js"

	enginewasm "m31labs.dev/gosx/engine/wasm"
)

const componentName = "GoTreesitterLangSearch"

func main() {
	if err := enginewasm.Register(componentName, mountLangSearch); err != nil {
		panic(err)
	}
	select {}
}

type langSearch struct {
	root    js.Value
	input   js.Value
	count   js.Value
	tiles   []js.Value
	onInput js.Func
	dispose sync.Once
	total   int
}

func mountLangSearch(ctx enginewasm.Context) (enginewasm.Handle, error) {
	mount := ctx.Mount()
	if !mount.Truthy() {
		return nil, fmt.Errorf("language-search engine mount is unavailable")
	}
	root := mount.Call("querySelector", ".lang-island")
	if !root.Truthy() {
		return nil, fmt.Errorf("language-search fallback root is missing")
	}
	input := root.Call("querySelector", ".search")
	count := root.Call("querySelector", ".count")
	if !input.Truthy() || !count.Truthy() {
		return nil, fmt.Errorf("language-search controls are incomplete")
	}

	nodes := root.Call("querySelectorAll", ".langtile")
	search := &langSearch{root: root, input: input, count: count, total: nodes.Get("length").Int()}
	search.tiles = make([]js.Value, search.total)
	for index := range search.tiles {
		search.tiles[index] = nodes.Index(index)
	}
	search.onInput = js.FuncOf(func(js.Value, []js.Value) any {
		search.apply(input.Get("value").String())
		return nil
	})
	input.Call("addEventListener", "input", search.onInput)
	search.apply(input.Get("value").String())
	root.Get("dataset").Set("ready", "true")
	return enginewasm.HandleFunc(search.close), nil
}

func (s *langSearch) apply(query string) {
	visible := 0
	for _, tile := range s.tiles {
		name := tile.Call("querySelector", ".lname").Get("textContent").String()
		matched := languageMatchesQuery(name, query)
		tile.Get("classList").Call("toggle", "hidden", !matched)
		if matched {
			visible++
		}
	}
	s.count.Set("textContent", strconv.Itoa(visible)+" / "+strconv.Itoa(s.total))
}

func (s *langSearch) close() {
	s.dispose.Do(func() {
		s.input.Call("removeEventListener", "input", s.onInput)
		s.onInput.Release()
		for _, tile := range s.tiles {
			tile.Get("classList").Call("remove", "hidden")
		}
		s.count.Set("textContent", strconv.Itoa(s.total)+" / "+strconv.Itoa(s.total))
		s.root.Get("dataset").Delete("ready")
	})
}
