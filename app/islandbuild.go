package docs

// Phase B2 — client islands.
//
// GoSX islands hydrate from a compiled `*program.Program`: a flat table of
// Nodes/Exprs interpreted by a small client-side VM (m31labs.dev/gosx/client/vm,
// compiled to WASM) that patches only the island's own DOM subtree. The
// production authoring path is `.gsx` source with a `//gosx:island` directive
// compiled through GoSX's own parser/lowerer; this file instead hand-builds
// the `program.Program` directly in Go, using only the long-stable opcode
// surface (signals, props, forEach, conditionals, string/collection ops) —
// see design/PHASE-B-NOTES.md and the worker report for why. `progBuilder`
// is the bookkeeping helper both islands (app/lang_island.go,
// app/playground_island.go) share so NodeID/ExprID arithmetic never has to
// be hand-computed.
//
// This is the REAL hydration mechanism — data-gosx-island + a compiled
// program served from /gosx/islands/*.json — not inline `onclick=""`
// attributes (which Phase A found bypass island codegen entirely).

import (
	"strconv"

	islandprogram "m31labs.dev/gosx/island/program"
)

type exprID = islandprogram.ExprID
type nodeID = islandprogram.NodeID

// progBuilder accumulates Nodes/Exprs for a hand-authored island Program.
// StaticMask is intentionally left nil: an absent (or false) StaticMask
// entry means "always reconcile" (see client/vm/vm.go's isDynamicSource),
// which is the safe default for islands this small — every node/attr is
// always re-checked in Reconcile() rather than trusting a hand-maintained
// static/dynamic split.
type progBuilder struct {
	nodes    []islandprogram.Node
	exprs    []islandprogram.Expr
	handlers []islandprogram.Handler
	signals  []islandprogram.SignalDef

	litStrCache  map[string]exprID
	litBoolCache map[bool]exprID
}

func newProgBuilder() *progBuilder {
	return &progBuilder{
		litStrCache:  make(map[string]exprID),
		litBoolCache: make(map[bool]exprID),
	}
}

func (b *progBuilder) addExpr(e islandprogram.Expr) exprID {
	id := exprID(len(b.exprs))
	b.exprs = append(b.exprs, e)
	return id
}

func (b *progBuilder) addNode(n islandprogram.Node) nodeID {
	id := nodeID(len(b.nodes))
	b.nodes = append(b.nodes, n)
	return id
}

// ---- expressions --------------------------------------------------------

func (b *progBuilder) lit(s string) exprID {
	if id, ok := b.litStrCache[s]; ok {
		return id
	}
	id := b.addExpr(islandprogram.Expr{Op: islandprogram.OpLitString, Value: s, Type: islandprogram.TypeString})
	b.litStrCache[s] = id
	return id
}

func (b *progBuilder) litBool(v bool) exprID {
	if id, ok := b.litBoolCache[v]; ok {
		return id
	}
	val := "false"
	if v {
		val = "true"
	}
	id := b.addExpr(islandprogram.Expr{Op: islandprogram.OpLitBool, Value: val, Type: islandprogram.TypeBool})
	b.litBoolCache[v] = id
	return id
}

func (b *progBuilder) litInt(n int) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpLitInt, Value: strconv.Itoa(n), Type: islandprogram.TypeInt})
}

func (b *progBuilder) signalGet(name string, typ islandprogram.ExprType) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpSignalGet, Value: name, Type: typ})
}

func (b *progBuilder) propGet(name string) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpPropGet, Value: name, Type: islandprogram.TypeAny})
}

func (b *progBuilder) index(coll, key exprID) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpIndex, Operands: []exprID{coll, key}, Type: islandprogram.TypeAny})
}

// field reads a named field off an object-shaped expr (e.g. a forEach item).
func (b *progBuilder) field(coll exprID, name string) exprID {
	return b.index(coll, b.lit(name))
}

func (b *progBuilder) contains(hay, needle exprID) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpContains, Operands: []exprID{hay, needle}, Type: islandprogram.TypeBool})
}

func (b *progBuilder) toLower(s exprID) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpToLower, Operands: []exprID{s}, Type: islandprogram.TypeString})
}

// concat folds 2+ string exprs pairwise: the VM's OpConcat is strictly
// binary (client/vm/vm.go's evalStringExpr routes it through evalBinary,
// which only reads Operands[0]/[1] — a 3-operand OpConcat like an older
// fixture used silently drops the third operand). Nesting avoids that trap.
func (b *progBuilder) concat(parts ...exprID) exprID {
	if len(parts) == 0 {
		return b.lit("")
	}
	acc := parts[0]
	for _, p := range parts[1:] {
		acc = b.addExpr(islandprogram.Expr{Op: islandprogram.OpConcat, Operands: []exprID{acc, p}, Type: islandprogram.TypeString})
	}
	return acc
}

func (b *progBuilder) cond(c, t, f exprID) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpCond, Operands: []exprID{c, t, f}, Type: islandprogram.TypeAny})
}

func (b *progBuilder) eq(a, c exprID) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpEq, Operands: []exprID{a, c}, Type: islandprogram.TypeBool})
}

func (b *progBuilder) neq(a, c exprID) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpNeq, Operands: []exprID{a, c}, Type: islandprogram.TypeBool})
}

func (b *progBuilder) and(a, c exprID) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpAnd, Operands: []exprID{a, c}, Type: islandprogram.TypeBool})
}

func (b *progBuilder) not(a exprID) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpNot, Operands: []exprID{a}, Type: islandprogram.TypeBool})
}

func (b *progBuilder) add(a, c exprID) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpAdd, Operands: []exprID{a, c}, Type: islandprogram.TypeInt})
}

func (b *progBuilder) length(a exprID) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpLen, Operands: []exprID{a}, Type: islandprogram.TypeInt})
}

func (b *progBuilder) filter(coll, pred exprID) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpFilter, Operands: []exprID{coll, pred}, Type: islandprogram.TypeAny})
}

func (b *progBuilder) split(s exprID, sep string) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpSplit, Operands: []exprID{s}, Value: sep, Type: islandprogram.TypeAny})
}

func (b *progBuilder) eventGet(field string) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpEventGet, Value: field, Type: islandprogram.TypeString})
}

func (b *progBuilder) signalSet(name string, value exprID, typ islandprogram.ExprType) exprID {
	return b.addExpr(islandprogram.Expr{Op: islandprogram.OpSignalSet, Operands: []exprID{value}, Value: name, Type: typ})
}

// ---- nodes ---------------------------------------------------------------

func attrStatic(name, val string) islandprogram.Attr {
	return islandprogram.Attr{Kind: islandprogram.AttrStatic, Name: name, Value: val}
}

func attrExpr(name string, e exprID) islandprogram.Attr {
	return islandprogram.Attr{Kind: islandprogram.AttrExpr, Name: name, Expr: e}
}

// attrEvent binds a DOM event (lowercase — "click", "input", ...) to a
// named handler, matching the convention the framework's own reference
// fixtures use (island/program/fixtures.go's CounterProgram etc.).
func attrEvent(domEvent, handler string) islandprogram.Attr {
	return islandprogram.Attr{Kind: islandprogram.AttrEvent, Name: domEvent, Event: handler}
}

func (b *progBuilder) el(tag string, attrs []islandprogram.Attr, children ...nodeID) nodeID {
	return b.addNode(islandprogram.Node{Kind: islandprogram.NodeElement, Tag: tag, Attrs: attrs, Children: children})
}

func (b *progBuilder) text(s string) nodeID {
	return b.addNode(islandprogram.Node{Kind: islandprogram.NodeText, Text: s})
}

func (b *progBuilder) exprNode(e exprID) nodeID {
	return b.addNode(islandprogram.Node{Kind: islandprogram.NodeExpr, Expr: e})
}

// condNode renders `children` when e is truthy, nothing otherwise (no
// fallback attr registered, which client/vm/vm.go's fallbackExpr treats as
// "render nothing" rather than an error).
func (b *progBuilder) condNode(e exprID, children ...nodeID) nodeID {
	return b.addNode(islandprogram.Node{Kind: islandprogram.NodeConditional, Expr: e, Children: children})
}

// forEachNode iterates the array/object expr e, binding each entry to the
// prop name `as` (readable from child exprs via propGet(as)) — see
// client/vm/vm.go's resolveForEachScope/bindForEachEntry.
func (b *progBuilder) forEachNode(e exprID, as string, children ...nodeID) nodeID {
	return b.addNode(islandprogram.Node{
		Kind:     islandprogram.NodeForEach,
		Expr:     e,
		Attrs:    []islandprogram.Attr{attrStatic("as", as)},
		Children: children,
	})
}

func (b *progBuilder) handler(name string, body ...exprID) {
	b.handlers = append(b.handlers, islandprogram.Handler{Name: name, Body: body})
}

func (b *progBuilder) signal(name string, typ islandprogram.ExprType, init exprID) {
	b.signals = append(b.signals, islandprogram.SignalDef{Name: name, Type: typ, Init: init})
}

func (b *progBuilder) build(name string, root nodeID) *islandprogram.Program {
	return &islandprogram.Program{
		Name:     name,
		Nodes:    b.nodes,
		Root:     root,
		Exprs:    b.exprs,
		Signals:  b.signals,
		Handlers: b.handlers,
	}
}
