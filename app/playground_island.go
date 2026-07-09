package docs

// Island 1 — the playground (design/PHASE-B-NOTES.md).
//
// State: { sample: "go"|"json"|"python", query: string, edited: bool } —
// exactly three signals. The code + tree panes are the island's own
// client-rendered output (not server HTML the island merely wraps): each
// sample's tokenized source and syntax tree are baked into the compiled
// Program as three sibling NodeConditional subtrees (one per sample, gated
// on `sample == "<id>"`), matching how the framework's own island VM
// renders "pick exactly one of several fixed subtrees" (see
// client/vm/vm.go's appendConditional — a NodeConditional has no native
// if/else-with-two-subtrees form, so three mutually-exclusive conditionals
// stand in for a switch).
//
// Query matcher (deliberately simple, no real query engine, per
// PHASE-B-NOTES): a tree row highlights when the live query signal
// contains the literal substring "(<row.Type>" — equivalent to the
// design's regex-extracted-type-set membership test for the single-type,
// no-whitespace-after-paren queries every preset uses. The capture badge is
// a documented simplification: instead of pairing each matched type with
// the *specific* trailing @capture the design's regex scan would find, one
// shared "first @word in the query" value (extracted via Split/Index —
// Contains-gated so the Index call is only reached when a "@" is present,
// keeping every Index in-bounds) badges every matched, unmarked row. See
// the worker report for why: OpConcat is strictly binary and there is no
// IndexOf primitive, so a faithful per-type capture map isn't expressible
// in this VM's stable opcode set without a real regex/scan opcode.

import (
	"fmt"
	"sync"

	"m31labs.dev/gosx"
	islandprogram "m31labs.dev/gosx/island/program"
	"m31labs.dev/gosx/server"
)

const playgroundIslandName = "Playground"

// PlaygroundProgramPath is where the compiled program is served from —
// mounted in main.go, referenced here via SetProgramAsset.
const PlaygroundProgramPath = "/gosx/islands/Playground.json"

var (
	playgroundProgramOnce sync.Once
	playgroundProgramVal  *islandprogram.Program
)

// PlaygroundProgram returns the (process-wide constant) compiled island
// program for the playground. Exported so main.go can serve it at
// playgroundProgramPath without rebuilding it per request.
func PlaygroundProgram() *islandprogram.Program {
	playgroundProgramOnce.Do(func() {
		playgroundProgramVal = buildPlaygroundProgram()
	})
	return playgroundProgramVal
}

// BuildPlaygroundIsland wires the playground island into a page: registers
// the compiled program's asset location on the page's runtime and returns
// the server-rendered, hydration-ready island node. Takes no props — every
// sample's data is baked into the compiled Program itself (see the file
// doc comment), so the page only needs the program, not per-request data.
func BuildPlaygroundIsland(rt *server.PageRuntime) gosx.Node {
	if rt == nil {
		return gosx.Text("")
	}
	rt.SetProgramAsset(playgroundIslandName, PlaygroundProgramPath, "json", "")
	return rt.Island(PlaygroundProgram(), nil)
}

func sampleByID(id string) playgroundSample {
	for _, s := range playgroundSamples {
		if s.ID == id {
			return s
		}
	}
	return playgroundSamples[0]
}

// nestedSampleCond builds a right-nested Cond chain selecting valueFor(id)
// for whichever sample the "sample" signal currently names, defaulting to
// the last sample if somehow none match (the signal is always one of the
// three IDs in practice — enum-shaped but the VM has no enum type).
func nestedSampleCond(b *progBuilder, valueFor func(sampleID string) exprID) exprID {
	n := len(playgroundSamples)
	acc := valueFor(playgroundSamples[n-1].ID)
	for i := n - 2; i >= 0; i-- {
		s := playgroundSamples[i]
		test := b.eq(b.signalGet("sample", islandprogram.TypeString), b.lit(s.ID))
		acc = b.cond(test, valueFor(s.ID), acc)
	}
	return acc
}

func buildPlaygroundProgram() *islandprogram.Program {
	b := newProgBuilder()

	b.signal("sample", islandprogram.TypeString, b.lit("go"))
	b.signal("query", islandprogram.TypeString, b.lit(""))
	b.signal("edited", islandprogram.TypeBool, b.litBool(false))

	b.handler("setQuery", b.signalSet("query", b.eventGet("value"), islandprogram.TypeString))
	b.handler("toggleEdit", b.signalSet("edited", b.not(b.signalGet("edited", islandprogram.TypeBool)), islandprogram.TypeBool))
	for _, s := range playgroundSamples {
		b.handler("select_"+s.ID,
			b.signalSet("sample", b.lit(s.ID), islandprogram.TypeString),
			b.signalSet("edited", b.litBool(false), islandprogram.TypeBool),
			b.signalSet("query", b.lit(""), islandprogram.TypeString),
		)
	}
	for _, s := range playgroundSamples {
		for i, preset := range s.Presets {
			b.handler(fmt.Sprintf("preset_%s_%d", s.ID, i), b.signalSet("query", b.lit(preset), islandprogram.TypeString))
		}
	}

	// ---- .seg sample switch -----------------------------------------
	var segButtons []nodeID
	for _, s := range playgroundSamples {
		on := b.eq(b.signalGet("sample", islandprogram.TypeString), b.lit(s.ID))
		class := b.cond(on, b.lit("segbtn on"), b.lit("segbtn"))
		segButtons = append(segButtons, b.el("button", []islandprogram.Attr{
			attrExpr("class", class),
			attrStatic("type", "button"),
			attrEvent("click", "select_"+s.ID),
		}, b.text(s.Label)))
	}
	seg := b.el("div", []islandprogram.Attr{attrStatic("class", "seg")}, segButtons...)

	// ---- .endpoint (static) -------------------------------------------
	endpoint := b.el("div", []islandprogram.Attr{attrStatic("class", "endpoint")},
		b.el("span", []islandprogram.Attr{attrStatic("class", "live")}),
		b.el("span", []islandprogram.Attr{attrStatic("class", "verb")}, b.text("POST")),
		b.text(" /parse → "),
		b.el("span", []islandprogram.Attr{attrStatic("class", "t-violet")}, b.text("gotreesitter")),
	)

	// ---- shared capture-badge extraction ------------------------------
	// hasAt gates the Index calls below: Split always returns >=1 element,
	// so Index(splitResult, 1) is only safe once hasAt guarantees a second
	// element exists — Cond is lazy (client/vm/vm.go's conditionalValue
	// only evaluates the taken branch), so the unsafe branch never runs
	// when hasAt is false.
	query := b.signalGet("query", islandprogram.TypeString)
	hasAt := b.contains(query, b.lit("@"))
	afterAt := b.index(b.split(query, "@"), b.litInt(1))
	beforeParen := b.index(b.split(afterAt, ")"), b.litInt(0))
	captureTok := b.index(b.split(beforeParen, " "), b.litInt(0))
	captureExpr := b.cond(hasAt, captureTok, b.lit(""))

	// ---- per-sample tree + code + match count -------------------------
	treeBySample := map[string]nodeID{}
	codeBySample := map[string]nodeID{}
	countBySample := map[string]exprID{}
	for _, s := range playgroundSamples {
		var matches []exprID
		treeBySample[s.ID] = buildTreeRow(b, s.Tree, captureExpr, &matches)
		countBySample[s.ID] = sumMatchCount(b, matches)
		codeBySample[s.ID] = b.el("pre", []islandprogram.Attr{attrStatic("class", "codebody")}, buildCodeTokens(b, s.Code)...)
	}
	matchCountExpr := nestedSampleCond(b, func(id string) exprID { return countBySample[id] })

	// ---- .querybar ------------------------------------------------
	querybar := b.el("div", []islandprogram.Attr{attrStatic("class", "querybar")},
		b.el("span", []islandprogram.Attr{attrStatic("class", "qlabel")}, b.text("query")),
		b.el("input", []islandprogram.Attr{
			attrStatic("class", "qinput mono"),
			attrStatic("type", "text"),
			attrStatic("placeholder", "(identifier) @name"),
			attrStatic("aria-label", "S-expression query"),
			attrExpr("value", query),
			attrEvent("input", "setQuery"),
		}),
		b.el("span", []islandprogram.Attr{attrStatic("class", "qcount")}, b.exprNode(matchCountExpr), b.text(" matched")),
	)

	// ---- .qpresets: one sibling row per sample, gated on sample -------
	var presetRows []nodeID
	for _, s := range playgroundSamples {
		var chips []nodeID
		for i, preset := range s.Presets {
			chips = append(chips, b.el("button", []islandprogram.Attr{
				attrStatic("class", "qchip"),
				attrStatic("type", "button"),
				attrEvent("click", fmt.Sprintf("preset_%s_%d", s.ID, i)),
			}, b.text(preset)))
		}
		test := b.eq(b.signalGet("sample", islandprogram.TypeString), b.lit(s.ID))
		presetRows = append(presetRows, b.condNode(test, chips...))
	}
	qpresets := b.el("div", []islandprogram.Attr{attrStatic("class", "qpresets")}, presetRows...)

	// ---- .playgrid: source + tree panels, gated on sample -------------
	var codePanes, treePanes []nodeID
	for _, s := range playgroundSamples {
		test := b.eq(b.signalGet("sample", islandprogram.TypeString), b.lit(s.ID))
		codePanes = append(codePanes, b.condNode(test, codeBySample[s.ID]))
		treePanes = append(treePanes, b.condNode(test, treeBySample[s.ID]))
	}
	codePanel := b.el("div", []islandprogram.Attr{attrStatic("class", "panel")},
		b.el("div", []islandprogram.Attr{attrStatic("class", "panelhd")},
			b.el("span", []islandprogram.Attr{attrStatic("class", "cdot r")}),
			b.el("span", []islandprogram.Attr{attrStatic("class", "cdot y")}),
			b.el("span", []islandprogram.Attr{attrStatic("class", "cdot g")}),
			b.text(" source"),
			b.el("span", []islandprogram.Attr{attrStatic("class", "hlcredit")}, b.text("NewHighlighter")),
		),
		b.el("div", []islandprogram.Attr{attrStatic("class", "panelbd"), attrStatic("style", "background:#17140f")}, codePanes...),
	)
	treePanel := b.el("div", []islandprogram.Attr{attrStatic("class", "panel")},
		b.el("div", []islandprogram.Attr{attrStatic("class", "panelhd")},
			b.el("span", []islandprogram.Attr{attrStatic("class", "ldot c-violet"), attrStatic("style", "border-color:var(--paper)")}),
			b.text(" syntax tree"),
			b.el("span", []islandprogram.Attr{attrStatic("class", "hlcredit")}, b.text("RootNode.SExpr")),
		),
		b.el("div", []islandprogram.Attr{attrStatic("class", "panelbd")},
			b.el("div", []islandprogram.Attr{attrStatic("class", "tree")}, treePanes...),
		),
	)
	playgrid := b.el("div", []islandprogram.Attr{attrStatic("class", "playgrid")}, codePanel, treePanel)

	// ---- .statstrip ----------------------------------------------------
	statBox := func(value exprID, color, label string) nodeID {
		svClass := "sv"
		if color != "" {
			svClass = "sv " + color
		}
		return b.el("div", []islandprogram.Attr{attrStatic("class", "stat")},
			b.el("div", []islandprogram.Attr{attrStatic("class", svClass)}, b.exprNode(value)),
			b.el("div", []islandprogram.Attr{attrStatic("class", "sl")}, b.text(label)),
		)
	}
	statTime := nestedSampleCond(b, func(id string) exprID { return b.lit(sampleByID(id).StatTime) })
	statNodes := nestedSampleCond(b, func(id string) exprID { return b.lit(sampleByID(id).StatNodes) })
	statBytes := nestedSampleCond(b, func(id string) exprID { return b.lit(sampleByID(id).StatBytes) })
	statAllocs := nestedSampleCond(b, func(id string) exprID { return b.lit(sampleByID(id).StatAllocs) })
	statstrip := b.el("div", []islandprogram.Attr{attrStatic("class", "statstrip")},
		statBox(statTime, "t-green", "Parse time"),
		statBox(statNodes, "t-blue", "Nodes"),
		statBox(statBytes, "", "Source"),
		statBox(statAllocs, "t-orange", "Allocs"),
	)

	// ---- .btnrow: incremental-reuse toggle ------------------------
	editLabel := b.cond(b.signalGet("edited", islandprogram.TypeBool), b.lit("Reset"), b.lit("Type an edit ▸"))
	btnrow := b.el("div", []islandprogram.Attr{attrStatic("class", "btnrow")},
		b.el("button", []islandprogram.Attr{
			attrStatic("class", "btn"),
			attrStatic("type", "button"),
			attrEvent("click", "toggleEdit"),
		}, b.exprNode(editLabel)),
	)

	incNote := b.condNode(b.signalGet("edited", islandprogram.TypeBool),
		b.el("div", []islandprogram.Attr{attrStatic("class", "incnote")},
			b.text("Edited the string literal. "),
			b.el("b", []islandprogram.Attr{attrStatic("class", "mono")}, b.text("ParseIncremental")),
			b.text(" re-lexed only that span — the pink node — and reused every green subtree by reference. Cost: "),
			b.el("b", []islandprogram.Attr{attrStatic("class", "mono")}, b.text("~649 ns")),
			b.text(", 3 allocs. Native C: ~102 µs."),
		),
	)
	tipNote := b.condNode(b.not(b.signalGet("edited", islandprogram.TypeBool)),
		b.el("div", []islandprogram.Attr{attrStatic("class", "note tip")},
			b.el("span", []islandprogram.Attr{attrStatic("class", "tag")}, b.text("Try it")),
			b.el("p", []islandprogram.Attr{attrStatic("class", "p"), attrStatic("style", "margin:0")},
				b.text("Hit "),
				b.el("b", nil, b.text("Type an edit")),
				b.text(" to change the string literal. The tree won't rebuild — only the touched subtree re-lexes, and the reused nodes glow green."),
			),
		),
	)

	foot := b.el("div", []islandprogram.Attr{attrStatic("class", "foot")},
		b.el("span", nil, b.text("gotreesitter · playground")),
		b.el("span", nil, b.text("trees animate as they assemble")),
	)

	credit := b.el("p", []islandprogram.Attr{attrStatic("class", "p mut"), attrStatic("style", "font-size:13.5px;margin-top:10px")},
		b.text("Every tree and highlight on this site is produced by gotreesitter parsing the snippet server-side — the docs for a parser, highlighted by the parser."),
	)

	root := b.el("div", []islandprogram.Attr{attrStatic("class", "playground-island")},
		seg, endpoint, querybar, qpresets, playgrid, credit, statstrip, btnrow, incNote, tipNote, foot,
	)

	return b.build(playgroundIslandName, root)
}

// buildCodeTokens renders one sample's tokenized source into span/br/text
// children for a <pre class="codebody">. Fully static — no signal/prop
// dependency — since the token list is fixed per sample and sample
// selection is handled by the sibling NodeConditional that wraps this.
func buildCodeTokens(b *progBuilder, tokens []codeTok) []nodeID {
	kids := make([]nodeID, 0, len(tokens))
	for _, t := range tokens {
		switch {
		case t.Newline:
			kids = append(kids, b.el("br", nil))
		case t.Class == "":
			kids = append(kids, b.text(t.Text))
		default:
			kids = append(kids, b.el("span", []islandprogram.Attr{attrStatic("class", t.Class)}, b.text(t.Text)))
		}
	}
	return kids
}

// buildTreeRow recursively renders one syntax-tree row (design's
// `.tnode > .trow + .tnest`) and appends this row's own query-match
// boolean expr to *matches so the caller can fold a total match count.
func buildTreeRow(b *progBuilder, row treeRow, captureExpr exprID, matches *[]exprID) nodeID {
	match := b.contains(b.signalGet("query", islandprogram.TypeString), b.concat(b.lit("("), b.lit(row.Type)))
	*matches = append(*matches, match)

	var marked exprID
	markClass := "trow"
	if row.Mark != "" {
		marked = b.signalGet("edited", islandprogram.TypeBool)
		markClass = "trow " + row.Mark
	} else {
		marked = b.litBool(false)
	}
	class := b.cond(marked, b.lit(markClass), b.cond(match, b.lit("trow qmatch"), b.lit("trow")))

	var rowKids []nodeID
	if row.Field != "" {
		rowKids = append(rowKids, b.el("span", []islandprogram.Attr{attrStatic("class", "tfield")}, b.text(row.Field+":")))
	}
	rowKids = append(rowKids, b.el("span", []islandprogram.Attr{attrStatic("class", "ttype")}, b.text(row.Type)))
	if row.Text != "" {
		rowKids = append(rowKids, b.el("span", []islandprogram.Attr{attrStatic("class", "ttext")}, b.text(row.Text)))
	}

	showBadge := b.and(b.not(marked), b.and(match, b.neq(captureExpr, b.lit(""))))
	rowKids = append(rowKids, b.condNode(showBadge,
		b.el("span", []islandprogram.Attr{attrStatic("class", "qcap")}, b.exprNode(b.concat(b.lit("@"), captureExpr))),
	))

	rowNode := b.el("div", []islandprogram.Attr{attrExpr("class", class)}, rowKids...)

	if len(row.Children) == 0 {
		return b.el("div", []islandprogram.Attr{attrStatic("class", "tnode")}, rowNode)
	}
	nestKids := make([]nodeID, 0, len(row.Children))
	for _, c := range row.Children {
		nestKids = append(nestKids, buildTreeRow(b, c, captureExpr, matches))
	}
	nest := b.el("div", []islandprogram.Attr{attrStatic("class", "tnest")}, nestKids...)
	return b.el("div", []islandprogram.Attr{attrStatic("class", "tnode")}, rowNode, nest)
}

// sumMatchCount folds a sample's per-row match booleans into a single
// Add(Cond(m,1,0), Add(Cond(m,1,0), ...)) count expr.
func sumMatchCount(b *progBuilder, matches []exprID) exprID {
	if len(matches) == 0 {
		return b.litInt(0)
	}
	acc := b.cond(matches[0], b.litInt(1), b.litInt(0))
	for _, m := range matches[1:] {
		acc = b.add(acc, b.cond(m, b.litInt(1), b.litInt(0)))
	}
	return acc
}
