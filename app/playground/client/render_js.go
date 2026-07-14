//go:build js && wasm

package main

import (
	"fmt"
	"strconv"
	"strings"
	"syscall/js"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

func (p *playground) renderPlain(source string) {
	p.ui.highlight.Set("textContent", source+"\n")
	p.syncScroll()
}

func (p *playground) syncScroll() {
	p.ui.highlight.Set("scrollTop", p.ui.source.Get("scrollTop"))
	p.ui.highlight.Set("scrollLeft", p.ui.source.Get("scrollLeft"))
}

func (p *playground) repaint() {
	if !p.hasRendered {
		p.renderPlain(p.ui.source.Get("value").String())
		return
	}
	bytes := []byte(p.renderedSource)
	classes := make([]string, len(bytes))
	for _, sourceRange := range p.tokenRanges {
		class := tokenClass(sourceRange.capture)
		if class == "" || sourceRange.end <= sourceRange.start {
			continue
		}
		start, end := min(int(sourceRange.start), len(bytes)), min(int(sourceRange.end), len(bytes))
		for index := max(0, start); index < end; index++ {
			classes[index] = class
		}
	}
	if p.queryOpen && p.captures != nil && p.capturesFor == p.renderedSource {
		for _, capture := range p.captures.ranges {
			if capture.end <= capture.start {
				continue
			}
			class := fmt.Sprintf("pg-capspan pg-cap-%d", capture.color)
			start, end := min(int(capture.start), len(bytes)), min(int(capture.end), len(bytes))
			for index := max(0, start); index < end; index++ {
				classes[index] = class
			}
		}
	}
	inFlash := func(index int) bool {
		return p.flash != nil && uint32(index) >= p.flash.start && uint32(index) < p.flash.end
	}

	document := js.Global().Get("document")
	fragment := document.Call("createDocumentFragment")
	for index := 0; index < len(bytes); {
		class := classes[index]
		flash := inFlash(index)
		end := index + 1
		for end < len(bytes) && classes[end] == class && inFlash(end) == flash {
			end++
		}
		// Parser and query spans are UTF-8 boundaries. Widen defensively if a
		// future capture introduces a partial-codepoint boundary.
		for end < len(bytes) && !utf8.RuneStart(bytes[end]) {
			end++
		}
		text := string(bytes[index:end])
		fullClass := strings.TrimSpace(class)
		if flash {
			fullClass = strings.TrimSpace(fullClass + " pg-flash")
		}
		if fullClass == "" {
			fragment.Call("appendChild", document.Call("createTextNode", text))
		} else {
			span := document.Call("createElement", "span")
			span.Set("className", fullClass)
			span.Set("textContent", text)
			fragment.Call("appendChild", span)
		}
		index = end
	}
	fragment.Call("appendChild", document.Call("createTextNode", "\n"))
	p.ui.highlight.Call("replaceChildren", fragment)
	p.syncScroll()
}

func (p *playground) flashSpan(start, end uint32) {
	p.flash = &flashRange{start: start, end: end}
	p.repaint()
	p.flashSeq++
	sequence := p.flashSeq
	go func() {
		time.Sleep(800 * time.Millisecond)
		if p.disposed || sequence != p.flashSeq {
			return
		}
		p.flash = nil
		p.repaint()
	}()
}

func (p *playground) nodeVisible(node *viewNode) bool {
	return node != nil && (node.named || p.showAnonymous || node.missing || node.isError)
}

func (p *playground) renderTree() int {
	document := js.Global().Get("document")
	fragment := document.Call("createDocumentFragment")
	p.visibleRows = p.visibleRows[:0]
	p.activeRow = js.Undefined()
	total := 0
	if p.treeRoot == nil {
		empty := document.Call("createElement", "div")
		empty.Set("className", "pg-tree-empty")
		empty.Set("textContent", "(empty tree)")
		fragment.Call("appendChild", empty)
	} else {
		type stackItem struct {
			node  *viewNode
			depth int
		}
		stack := []stackItem{{node: p.treeRoot}}
		for len(stack) > 0 {
			item := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			childDepth := item.depth
			if p.nodeVisible(item.node) {
				total++
				childDepth++
				if len(p.visibleRows) < maxTreeRows {
					row := document.Call("createElement", "div")
					class := "pg-tline"
					if item.node.isError {
						class += " pg-err"
					}
					if item.node.missing {
						class += " pg-missingrow"
					}
					if !item.node.named {
						class += " pg-anonrow"
					}
					row.Set("className", class)
					row.Get("style").Set("paddingLeft", fmt.Sprintf("%dpx", item.depth*14))
					row.Get("dataset").Set("i", strconv.Itoa(len(p.visibleRows)))
					row.Get("dataset").Set("s16", strconv.FormatUint(uint64(item.node.start16), 10))
					row.Get("dataset").Set("e16", strconv.FormatUint(uint64(item.node.end16), 10))
					row.Call("setAttribute", "role", "treeitem")
					row.Call("setAttribute", "aria-level", strconv.Itoa(item.depth+1))
					row.Call("setAttribute", "aria-selected", "false")
					row.Set("tabIndex", -1)
					if item.node.field != "" {
						field := document.Call("createElement", "span")
						field.Set("className", "tfield")
						field.Set("textContent", item.node.field+": ")
						row.Call("appendChild", field)
					}
					typeName := document.Call("createElement", "span")
					typeName.Set("className", "ttype")
					label := item.node.typ
					if !item.node.named {
						label = strconv.Quote(label)
					}
					typeName.Set("textContent", label)
					row.Call("appendChild", typeName)
					if item.node.missing {
						missing := document.Call("createElement", "span")
						missing.Set("className", "tmissing")
						missing.Set("textContent", "MISSING")
						row.Call("appendChild", missing)
					}
					p.visibleRows = append(p.visibleRows, visibleRow{node: item.node, el: row})
					fragment.Call("appendChild", row)
				}
			}
			for index := len(item.node.children) - 1; index >= 0; index-- {
				stack = append(stack, stackItem{node: item.node.children[index], depth: childDepth})
			}
		}
		if total > len(p.visibleRows) {
			more := document.Call("createElement", "div")
			more.Set("className", "pg-tline pg-more")
			more.Set("textContent", fmt.Sprintf("… %d more nodes", total-len(p.visibleRows)))
			fragment.Call("appendChild", more)
		}
		if p.treeRoot.truncated {
			note := document.Call("createElement", "div")
			note.Set("className", "pg-tline pg-more")
			note.Set("textContent", "⚠ tree truncated at 20,000 nodes")
			fragment.Call("appendChild", note)
		}
	}
	if len(p.visibleRows) > 0 {
		p.treeFocusIndex = min(p.treeFocusIndex, len(p.visibleRows)-1)
		p.visibleRows[p.treeFocusIndex].el.Set("tabIndex", 0)
	} else {
		p.treeFocusIndex = 0
	}
	scrollTop := p.ui.tree.Get("scrollTop")
	p.ui.tree.Call("replaceChildren", fragment)
	p.ui.tree.Set("scrollTop", scrollTop)
	p.markTreeCaptures()
	return total
}

func (p *playground) deepestVisibleAt(caret int) *viewNode {
	node := p.treeRoot
	if node == nil || caret < int(node.start16) || caret > int(node.end16) {
		return nil
	}
	var best *viewNode
	for node != nil {
		if p.nodeVisible(node) && int(node.start16) <= caret && caret <= int(node.end16) {
			best = node
		}
		var next *viewNode
		for _, child := range node.children {
			if int(child.start16) <= caret && caret < int(child.end16) {
				next = child
				break
			}
		}
		if next == nil {
			for index := len(node.children) - 1; index >= 0; index-- {
				child := node.children[index]
				if child.start16 < child.end16 && int(child.start16) <= caret && caret == int(child.end16) {
					next = child
					break
				}
			}
		}
		node = next
	}
	return best
}

func (p *playground) handleTreeClick(event js.Value) {
	row := closestTreeRow(event)
	if !row.Truthy() {
		return
	}
	index, err := strconv.Atoi(row.Get("dataset").Get("i").String())
	if err != nil {
		return
	}
	p.setTreeFocusIndex(index, true)
	p.activateTreeRow(index)
}

func (p *playground) handleTreeFocus(event js.Value) {
	row := closestTreeRow(event)
	if !row.Truthy() {
		return
	}
	index, err := strconv.Atoi(row.Get("dataset").Get("i").String())
	if err == nil {
		p.setTreeFocusIndex(index, false)
	}
}

func (p *playground) handleTreeKeydown(event js.Value) {
	row := closestTreeRow(event)
	if !row.Truthy() {
		return
	}
	index, err := strconv.Atoi(row.Get("dataset").Get("i").String())
	if err != nil {
		return
	}
	switch event.Get("key").String() {
	case "ArrowDown":
		event.Call("preventDefault")
		p.setTreeFocusIndex(index+1, true)
	case "ArrowUp":
		event.Call("preventDefault")
		p.setTreeFocusIndex(index-1, true)
	case "Home":
		event.Call("preventDefault")
		p.setTreeFocusIndex(0, true)
	case "End":
		event.Call("preventDefault")
		p.setTreeFocusIndex(len(p.visibleRows)-1, true)
	case "Enter", " ":
		event.Call("preventDefault")
		p.activateTreeRow(index)
	}
}

func closestTreeRow(event js.Value) js.Value {
	if !event.Truthy() {
		return js.Undefined()
	}
	target := event.Get("target")
	if !target.Truthy() || target.Get("closest").Type() != js.TypeFunction {
		return js.Undefined()
	}
	return target.Call("closest", ".pg-tline[data-i]")
}

func (p *playground) setTreeFocusIndex(index int, focus bool) {
	if len(p.visibleRows) == 0 {
		return
	}
	index = max(0, min(index, len(p.visibleRows)-1))
	current := p.ui.tree.Call("querySelector", `.pg-tline[tabindex="0"]`)
	if current.Truthy() {
		current.Set("tabIndex", -1)
	}
	p.treeFocusIndex = index
	row := p.visibleRows[index].el
	row.Set("tabIndex", 0)
	if focus {
		row.Call("focus")
	}
	p.scrollTreeRowIntoView(row)
}

func (p *playground) scrollTreeRowIntoView(row js.Value) {
	container := p.ui.tree.Call("getBoundingClientRect")
	item := row.Call("getBoundingClientRect")
	if item.Get("top").Float() < container.Get("top").Float() {
		p.ui.tree.Set("scrollTop", p.ui.tree.Get("scrollTop").Float()+item.Get("top").Float()-container.Get("top").Float()-36)
	} else if item.Get("bottom").Float() > container.Get("bottom").Float() {
		p.ui.tree.Set("scrollTop", p.ui.tree.Get("scrollTop").Float()+item.Get("bottom").Float()-container.Get("bottom").Float()+36)
	}
}

func (p *playground) highlightTreeNode(node *viewNode) {
	if p.activeRow.Truthy() {
		p.activeRow.Get("classList").Call("remove", "pg-active")
		p.activeRow.Call("setAttribute", "aria-selected", "false")
		p.activeRow = js.Undefined()
	}
	if node == nil {
		return
	}
	for _, row := range p.visibleRows {
		if row.node != node {
			continue
		}
		row.el.Get("classList").Call("add", "pg-active")
		row.el.Call("setAttribute", "aria-selected", "true")
		p.activeRow = row.el
		p.scrollTreeRowIntoView(row.el)
		return
	}
}

func (p *playground) activateTreeRow(index int) {
	if index < 0 || index >= len(p.visibleRows) {
		return
	}
	node := p.visibleRows[index].node
	p.selectNodeInEditor(node)
	p.highlightTreeNode(node)
}

func (p *playground) selectNodeInEditor(node *viewNode) {
	if node == nil {
		return
	}
	p.ui.source.Call("setSelectionRange", node.start16, node.end16)
	source := p.ui.source.Get("value").String()
	prefix := stringPrefixUTF16(source, int(node.start16))
	line := strings.Count(prefix, "\n")
	lineHeight := 23.0
	if parsed, err := strconv.ParseFloat(strings.TrimSuffix(js.Global().Call("getComputedStyle", p.ui.source).Get("lineHeight").String(), "px"), 64); err == nil && parsed > 0 {
		lineHeight = parsed
	}
	p.ui.source.Set("scrollTop", maxFloat(0, float64(line)*lineHeight-p.ui.source.Get("clientHeight").Float()*0.35))
	column := len([]rune(prefix[strings.LastIndex(prefix, "\n")+1:]))
	p.ui.source.Set("scrollLeft", maxFloat(0, float64(column)*8.1-p.ui.source.Get("clientWidth").Float()*0.3))
	p.syncScroll()
	p.flashSpan(node.start, node.end)
}

func stringPrefixUTF16(source string, units int) string {
	if units <= 0 {
		return ""
	}
	count := 0
	for byteIndex, r := range source {
		width := 1
		if r > 0xffff {
			width = 2
		}
		if count+width > units {
			return source[:byteIndex]
		}
		count += width
		if count == units {
			return source[:byteIndex+utf8.RuneLen(r)]
		}
	}
	return source
}

func (p *playground) runQuery() {
	if p.queryProbe != nil {
		p.runQueryProbe()
		return
	}
	if !p.ready || !p.queryOpen || p.disposed {
		return
	}
	querySource := p.ui.query.Get("value").String()
	if strings.TrimSpace(querySource) == "" {
		p.captures = nil
		p.capturesFor = ""
		p.clearQueryError()
		p.renderLegend(nil)
		p.markTreeCaptures()
		p.repaint()
		return
	}
	if !p.hasRendered || p.document.tree == nil {
		return
	}
	result := queryDocument(&p.document, querySource)
	if result.Err != nil {
		if p.captures != nil && p.capturesFor != p.renderedSource {
			p.captures = nil
			p.capturesFor = ""
			p.renderLegend(nil)
			p.markTreeCaptures()
			p.repaint()
		}
		p.showQueryError(result.Err.Error(), result.ErrOffset, p.captures != nil && len(p.captures.byName) > 0)
		return
	}
	p.clearQueryError()
	captures := &captureState{
		byName:    make(map[string]*captureInfo),
		nodeMarks: make(map[string][]captureMark),
		truncated: result.truncated,
	}
	for _, capture := range result.captures {
		info := captures.byName[capture.name]
		if info == nil {
			info = &captureInfo{color: captureColor(capture.name)}
			captures.byName[capture.name] = info
			captures.order = append(captures.order, capture.name)
		}
		info.count++
		captures.ranges = append(captures.ranges, captureRange{start: capture.start, end: capture.end, color: info.color})
		key := nodeMarkKey(capture.start, capture.end, capture.typ)
		marks := captures.nodeMarks[key]
		seen := false
		for _, mark := range marks {
			if mark.name == capture.name {
				seen = true
				break
			}
		}
		if !seen {
			captures.nodeMarks[key] = append(marks, captureMark{name: capture.name, color: info.color})
		}
	}
	p.captures = captures
	p.capturesFor = p.renderedSource
	p.renderLegend(captures)
	p.markTreeCaptures()
	p.repaint()
}

func (p *playground) clearQueryError() {
	p.ui.queryErr.Set("hidden", true)
	p.ui.queryErr.Call("replaceChildren")
}

func (p *playground) showQueryError(message string, byteOffset int, keptStale bool) {
	p.ui.queryErr.Set("hidden", false)
	p.ui.queryErr.Call("replaceChildren")
	document := js.Global().Get("document")
	messageNode := document.Call("createElement", "div")
	messageNode.Set("className", "pg-qerrmsg")
	messageNode.Set("textContent", message)
	p.ui.queryErr.Call("appendChild", messageNode)
	querySource := p.ui.query.Get("value").String()
	if byteOffset >= 0 {
		byteOffset = max(0, min(byteOffset, len(querySource)))
		for byteOffset > 0 && byteOffset < len(querySource) && !utf8.RuneStart(querySource[byteOffset]) {
			byteOffset--
		}
		lineStart := strings.LastIndex(querySource[:byteOffset], "\n") + 1
		lineEnd := strings.Index(querySource[byteOffset:], "\n")
		if lineEnd < 0 {
			lineEnd = len(querySource)
		} else {
			lineEnd += byteOffset
		}
		line := document.Call("createElement", "div")
		line.Set("className", "pg-qerrline")
		line.Set("textContent", querySource[lineStart:lineEnd])
		caret := document.Call("createElement", "div")
		caret.Set("className", "pg-qerrcaret")
		caret.Set("textContent", strings.Repeat(" ", len(utf16.Encode([]rune(querySource[lineStart:byteOffset]))))+"^")
		p.ui.queryErr.Call("appendChild", line)
		p.ui.queryErr.Call("appendChild", caret)
	}
	if keptStale {
		note := document.Call("createElement", "div")
		note.Set("className", "pg-qerrnote")
		note.Set("textContent", "captures from the last valid query are still shown")
		p.ui.queryErr.Call("appendChild", note)
	}
}

func (p *playground) renderLegend(captures *captureState) {
	if captures == nil || len(captures.byName) == 0 {
		p.ui.legend.Set("hidden", true)
		p.ui.legend.Call("replaceChildren")
		return
	}
	p.ui.legend.Set("hidden", false)
	document := js.Global().Get("document")
	fragment := document.Call("createDocumentFragment")
	for _, name := range captures.order {
		info := captures.byName[name]
		item := document.Call("createElement", "span")
		item.Set("className", "pg-legenditem")
		swatch := document.Call("createElement", "i")
		swatch.Set("className", fmt.Sprintf("pg-swatch pg-swatch-%d", info.color))
		item.Call("appendChild", swatch)
		label := document.Call("createElement", "span")
		label.Set("textContent", fmt.Sprintf("@%s ×%d", name, info.count))
		item.Call("appendChild", label)
		fragment.Call("appendChild", item)
	}
	if captures.truncated {
		truncated := document.Call("createElement", "span")
		truncated.Set("className", "pg-legendtrunc")
		truncated.Set("textContent", "truncated at 500 matches")
		fragment.Call("appendChild", truncated)
	}
	p.ui.legend.Call("replaceChildren", fragment)
}

func (p *playground) markTreeCaptures() {
	removeAll := func(selector string, remove func(js.Value)) {
		nodes := p.ui.tree.Call("querySelectorAll", selector)
		for index := 0; index < nodes.Get("length").Int(); index++ {
			remove(nodes.Index(index))
		}
	}
	removeAll(".pg-capbadge", func(value js.Value) { value.Call("remove") })
	removeAll(".pg-tline.pg-capped", func(value js.Value) { value.Get("classList").Call("remove", "pg-capped") })
	if !p.queryOpen || p.captures == nil || p.capturesFor != p.renderedSource {
		return
	}
	document := js.Global().Get("document")
	for _, row := range p.visibleRows {
		marks := p.captures.nodeMarks[nodeMarkKey(row.node.start, row.node.end, row.node.typ)]
		if len(marks) == 0 {
			continue
		}
		row.el.Get("classList").Call("add", "pg-capped")
		for _, mark := range marks {
			badge := document.Call("createElement", "span")
			badge.Set("className", "pg-capbadge")
			swatch := document.Call("createElement", "i")
			swatch.Set("className", fmt.Sprintf("pg-swatch pg-swatch-%d", mark.color))
			badge.Call("appendChild", swatch)
			badge.Call("appendChild", document.Call("createTextNode", "@"+mark.name))
			row.el.Call("appendChild", badge)
		}
	}
}

func nodeMarkKey(start, end uint32, typ string) string {
	return fmt.Sprintf("%d:%d:%s", start, end, typ)
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
