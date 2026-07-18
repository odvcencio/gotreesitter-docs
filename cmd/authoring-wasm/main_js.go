//go:build js && wasm

package main

import (
	"fmt"
	"sync"
	"syscall/js"
	"time"

	"github.com/odvcencio/gotreesitter-docs/internal/authoringengine"
	enginewasm "m31labs.dev/gosx/engine/wasm"
)

type browserListener struct {
	target js.Value
	event  string
	fn     js.Func
}

type authoringHandle struct {
	mount     js.Value
	listeners []browserListener

	stateMu  sync.Mutex
	timer    *time.Timer
	disposed bool
	runSeq   uint64
}

func main() {
	if err := enginewasm.Register("GotreesitterAuthoring", mountAuthoring); err != nil {
		panic(err)
	}
	select {}
}

// mountAuthoring wires the grammar-authoring surface: a grammar.json
// textarea (#ag-grammar), a sample-source textarea (#ag-source), an
// anonymous-node toggle (#ag-anonymous), and the render sinks
// (#ag-tree/#ag-errors/#ag-status/#ag-node-count). Unlike the parse
// playground (cmd/playground-wasm), this engine needs no bootstrap fetch —
// grammargen compiles the grammar live from the textarea, so it can run its
// first pass as soon as listeners are bound.
func mountAuthoring(ctx enginewasm.Context) (enginewasm.Handle, error) {
	mount := ctx.Mount()
	if !mount.Truthy() {
		return nil, fmt.Errorf("authoring surface requires a DOM mount")
	}
	h := &authoringHandle{mount: mount}
	for _, binding := range []struct {
		selector  string
		event     string
		immediate bool
	}{
		{"#ag-grammar", "input", false},
		{"#ag-source", "input", false},
		{"#ag-anonymous", "change", true},
	} {
		target := h.find(binding.selector)
		if !target.Truthy() {
			return nil, fmt.Errorf("authoring element %s is missing", binding.selector)
		}
		immediate := binding.immediate
		fn := js.FuncOf(func(js.Value, []js.Value) any {
			if immediate {
				h.run()
			} else {
				h.schedule()
			}
			return nil
		})
		target.Call("addEventListener", binding.event, fn)
		h.listeners = append(h.listeners, browserListener{target: target, event: binding.event, fn: fn})
	}
	h.mount.Get("dataset").Set("privacyBoundary", "browser-only")
	if seq, ok := h.beginRun(); ok {
		go h.runAsync(seq)
	}
	return h, nil
}

func (h *authoringHandle) Dispose() {
	h.stateMu.Lock()
	h.disposed = true
	h.runSeq++
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	h.stateMu.Unlock()
	for _, listener := range h.listeners {
		listener.target.Call("removeEventListener", listener.event, listener.fn)
		listener.fn.Release()
	}
	h.listeners = nil
}

func (h *authoringHandle) schedule() {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	if h.disposed {
		return
	}
	if h.timer != nil {
		h.timer.Stop()
	}
	h.timer = time.AfterFunc(180*time.Millisecond, h.run)
}

func (h *authoringHandle) run() {
	seq, ok := h.beginRun()
	if !ok {
		return
	}
	go h.runAsync(seq)
}

func (h *authoringHandle) beginRun() (uint64, bool) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	if h.disposed {
		return 0, false
	}
	h.timer = nil
	h.runSeq++
	return h.runSeq, true
}

func (h *authoringHandle) runAsync(seq uint64) {
	defer func() {
		if recovered := recover(); recovered != nil && h.current(seq) {
			h.fail(fmt.Sprintf("browser compiler failed: %v", recovered))
		}
	}()
	grammarJSON := h.find("#ag-grammar").Get("value").String()
	source := h.find("#ag-source").Get("value").String()
	includeAnonymous := true
	if el := h.find("#ag-anonymous"); el.Truthy() {
		includeAnonymous = el.Get("checked").Bool()
	}
	if !h.current(seq) {
		return
	}
	h.text("#ag-status", "Compiling grammar locally…")
	started := time.Now()
	result := authoringengine.Compile(grammarJSON, source, includeAnonymous)
	if h.current(seq) {
		h.render(result, time.Since(started))
	}
}

func (h *authoringHandle) current(seq uint64) bool {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	return !h.disposed && h.runSeq == seq
}

func (h *authoringHandle) fail(message string) {
	h.text("#ag-status", message)
	errors := h.find("#ag-errors")
	errors.Set("textContent", "")
	box := element("div")
	box.Set("className", "ag-err mono")
	box.Set("role", "alert")
	box.Set("textContent", message)
	errors.Call("appendChild", box)
}

func (h *authoringHandle) render(result authoringengine.Result, elapsed time.Duration) {
	h.text("#ag-node-count", fmt.Sprintf("%d nodes", result.NodeCount))

	tree := h.find("#ag-tree")
	tree.Set("textContent", "")
	for _, row := range result.TreeRows {
		line := element("div")
		line.Set("className", row.Class)
		line.Set("role", "treeitem")
		line.Call("setAttribute", "aria-level", fmt.Sprint(row.Level))
		line.Call("appendChild", textNode(row.Depth))
		if row.Field != "" {
			line.Call("appendChild", styledText("span", "tfield", row.Field+": "))
		}
		line.Call("appendChild", styledText("span", "ttype", row.Type))
		line.Call("appendChild", styledText("span", "tfield", " ["+row.Range+"]"))
		if row.Missing {
			line.Call("appendChild", styledText("span", "tmissing", "MISSING"))
		}
		tree.Call("appendChild", line)
	}
	if len(result.TreeRows) == 0 {
		empty := element("p")
		empty.Set("className", "ag-tree-empty")
		empty.Set("textContent", "No tree rows to display.")
		tree.Call("appendChild", empty)
	}

	errors := h.find("#ag-errors")
	errors.Set("textContent", "")
	for _, message := range []string{result.ImportError, result.GenerateError, result.ParseError} {
		if message == "" {
			continue
		}
		box := element("div")
		box.Set("className", "ag-err mono")
		box.Set("role", "alert")
		box.Set("textContent", message)
		errors.Call("appendChild", box)
	}

	switch {
	case result.ImportError != "":
		h.text("#ag-status", "grammar.json failed to import; see errors below.")
	case result.GenerateError != "":
		h.text("#ag-status", "Grammar failed to compile; see errors below.")
	case result.ParseError != "":
		h.text("#ag-status", "Sample failed to parse; see errors below.")
	case result.HasErrors:
		h.text("#ag-status", fmt.Sprintf("Compiled %q; sample has parse errors (%s).", result.GrammarName, elapsed.Round(100*time.Microsecond)))
	default:
		h.text("#ag-status", fmt.Sprintf("Compiled %q and parsed locally in %s.", result.GrammarName, elapsed.Round(100*time.Microsecond)))
	}
}

func (h *authoringHandle) find(selector string) js.Value {
	return h.mount.Call("querySelector", selector)
}

func (h *authoringHandle) text(selector, value string) {
	target := h.find(selector)
	if target.Truthy() {
		target.Set("textContent", value)
	}
}

func element(tag string) js.Value {
	return js.Global().Get("document").Call("createElement", tag)
}

func textNode(value string) js.Value {
	return js.Global().Get("document").Call("createTextNode", value)
}

func styledText(tag, className, value string) js.Value {
	node := element(tag)
	node.Set("className", className)
	node.Set("textContent", value)
	return node
}
