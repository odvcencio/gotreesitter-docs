package docs

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/odvcencio/mdpp"
	"m31labs.dev/gosx"
)

// dotPalette is public/docs.css's 8-color rotation (.c-violet/.c-blue/.c-cyan/
// .c-green/.c-yellow/.c-orange/.c-red/.c-pink), reused everywhere Phase B
// needs to color-cycle a generated sequence: step numbers, feature cards,
// and the static language grid.
var dotPalette = []string{
	"c-violet", "c-blue", "c-cyan", "c-green", "c-yellow", "c-orange", "c-red", "c-pink",
}

// tPalette is the matching text-color rotation (.t-violet etc.) used for
// .statstrip big numbers, which public/docs.css colors with `.t-*` rather than
// `.c-*` (`.stat .sv` is plain text, not a filled swatch).
var tPalette = []string{
	"t-violet", "t-green", "t-blue", "t-orange", "t-pink", "t-cyan",
}

// renderBlocks renders a run of top-level (or list-item, or admonition-body)
// block nodes. It also owns the one multi-node lookahead in this renderer:
// a pair of adjacent H3 sections whose headings are marked "✗ ..." / "✓ ..."
// (external-scanners.md's "do you need a scanner" split) collapses into the
// design's `.decide` two-card comparison instead of two independent H3
// sections — see tryDecidePair.
func renderBlocks(nodes []*mdpp.Node, src []byte) []gosx.Node {
	var out []gosx.Node
	for i := 0; i < len(nodes); i++ {
		if consumed, decide := tryDecidePair(nodes, i, src); consumed > 0 {
			out = append(out, decide)
			i += consumed - 1
			continue
		}
		out = append(out, renderBlock(nodes[i], src)...)
	}
	return out
}

// renderBlock renders one block node. It returns a slice because an `<h2>`
// expands into two design siblings: the heading itself plus the
// `.underbar` the site stylesheet always renders directly under it.
func renderBlock(n *mdpp.Node, src []byte) []gosx.Node {
	if n == nil {
		return nil
	}
	switch n.Type {
	case mdpp.NodeHeading:
		return renderHeading(n)
	case mdpp.NodeParagraph:
		return []gosx.Node{elWith("p", gosx.Attrs(gosx.Attr("class", "p")), renderInline(n.Children)...)}
	case mdpp.NodeCodeBlock:
		return []gosx.Node{renderCodeBlock(n, src)}
	case mdpp.NodeAdmonition:
		return []gosx.Node{renderAdmonition(n, src)}
	case mdpp.NodeBlockquote:
		return []gosx.Node{renderPlainQuote(n, src)}
	case mdpp.NodeList:
		return []gosx.Node{renderList(n, src)}
	case mdpp.NodeTable:
		return []gosx.Node{renderTable(n)}
	case mdpp.NodeThematicBreak:
		return []gosx.Node{elWith("hr", nil)}
	case mdpp.NodeHTMLBlock:
		// Self-authored content/docs/*.md — trusted raw HTML passthrough,
		// same trust boundary content.MDPPRenderer used before Phase B.
		return []gosx.Node{gosx.RawHTML(n.Literal)}
	case mdpp.NodeFrontmatter:
		return nil
	default:
		if len(n.Children) > 0 {
			return renderBlocks(n.Children, src)
		}
		return nil
	}
}

func renderHeading(n *mdpp.Node) []gosx.Node {
	level := n.Level()
	heading := elWith(
		headingTag(level),
		gosx.Attrs(gosx.Attr("class", headingClass(level)), gosx.Attr("id", mdpp.Slugify(n.Text()))),
		renderInline(n.Children)...,
	)
	if level <= 2 {
		return []gosx.Node{heading, elWith("div", gosx.Attrs(gosx.Attr("class", "underbar")))}
	}
	return []gosx.Node{heading}
}

// ---- code blocks -----------------------------------------------------

// shellLikeFenceLangs are fence languages the design shows with a bare
// `.cfile` label ("shell") and no `.clang` pill.
var shellLikeFenceLangs = map[string]bool{
	"sh": true, "shell": true, "bash": true, "zsh": true, "console": true,
}

// renderCodeBlock is the Phase B markdown-pipeline convention for fenced
// code:
//   - ```lang title=filename.ext → `.cfile` shows the filename, `.clang`
//     shows lang (skipped for shell-like/plain langs, matching the design's
//     own command blocks).
//   - ```lang (no title) → `.cfile` falls back to the language name itself
//     ("shell" for sh/bash/zsh, "output" for text/plain/untagged fences).
//   - ```langlist is a Phase-B-only pseudo-language (see languages.md) that
//     renders the static 206-language `.langgrid` instead of a code block.
//
// Highlighting: highlightSource (highlight.go) parses the snippet with the
// matching gotreesitter grammar and emits tk-* spans; a language gotreesitter
// doesn't recognize (or "text"/blank) falls back to a clean, unhighlighted
// `.codebody` rather than a render error — real highlighting where it can be
// real, never fake highlighting where it can't.
func renderCodeBlock(n *mdpp.Node, src []byte) gosx.Node {
	lang := strings.ToLower(strings.TrimSpace(n.Attrs["language"]))
	literal := strings.TrimSuffix(n.Literal, "\n")

	if lang == "langlist" {
		return renderLangGrid(literal)
	}

	title := fenceTitle(src, n.Range)
	isShell := shellLikeFenceLangs[lang]
	isPlain := lang == "" || lang == "text" || lang == "plain"

	cfile := title
	if cfile == "" {
		switch {
		case isShell:
			cfile = "shell"
		case isPlain:
			cfile = "output"
		default:
			cfile = lang
		}
	}

	var body gosx.Node
	switch {
	case isPlain:
		body = elWith("pre", gosx.Attrs(gosx.Attr("class", "codebody")), gosx.Text(literal))
	default:
		if highlighted, ok := highlightSource(lang, literal); ok {
			body = elWith("pre", gosx.Attrs(gosx.Attr("class", "codebody")), gosx.RawHTML(highlighted))
		} else {
			body = elWith("pre", gosx.Attrs(gosx.Attr("class", "codebody")), gosx.Text(literal))
		}
	}

	codeClass := "code"
	if isShell || isPlain {
		codeClass += " tight"
	}

	head := []gosx.Node{
		elWith("span", gosx.Attrs(gosx.Attr("class", "cdot r"))),
		elWith("span", gosx.Attrs(gosx.Attr("class", "cdot y"))),
		elWith("span", gosx.Attrs(gosx.Attr("class", "cdot g"))),
		elWith("span", gosx.Attrs(gosx.Attr("class", "cfile mono")), gosx.Text(cfile)),
	}
	if !isShell && !isPlain {
		head = append(head, elWith("span", gosx.Attrs(gosx.Attr("class", "clang")), gosx.Text(lang)))
	}

	return elWith("div", gosx.Attrs(gosx.Attr("class", codeClass)),
		elWith("div", gosx.Attrs(gosx.Attr("class", "codehead")), head...),
		body,
	)
}

// fenceTitle recovers the ```lang title=filename.ext convention. mdpp's own
// NodeCodeBlock.Attrs["language"] is already normalized down to just the
// first whitespace-delimited fence-info token (see mdpp's
// normalizedFenceLanguage), which silently drops everything after it — so
// the only way to see "title=main.go" is to re-read the raw fence-open line
// straight out of the document source using the node's own byte range.
func fenceTitle(src []byte, r mdpp.Range) string {
	if r.StartByte < 0 || r.StartByte >= len(src) {
		return ""
	}
	rest := src[r.StartByte:]
	if nl := bytes.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[:nl]
	}
	line := strings.TrimSpace(string(rest))
	line = strings.TrimLeft(line, "`~")
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return ""
	}
	for _, f := range fields[1:] {
		if v, ok := strings.CutPrefix(f, "title="); ok {
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}

// ---- language grid (languages.md's ```langlist block) ----------------

// langTokenSourceSet marks the languages languages.md's prose names as
// having a hand-written Go token source instead of the generated DFA lexer
// ("Non-nil only for languages with a hand-written Go token source instead
// of the DFA lexer (default build: c, cpp, java, json, authzed)" —
// content/docs/languages.md, "The registry: LangEntry and AllLanguages").
// Uses the site's `.lts` "TS" badge for the real token-source catalog.
var langTokenSourceSet = map[string]bool{
	"c": true, "cpp": true, "java": true, "json": true, "authzed": true,
}

// renderLangGrid renders the static (non-interactive) `.langgrid` wall of
// language tiles. The live search/filter input (design's Island 2) is
// explicitly out of scope for Phase B1 — this only ports the always-visible
// styled grid so the languages page doesn't read as a wall of monospace
// text while the filter island is still pending.
func renderLangGrid(literal string) gosx.Node {
	names := strings.Fields(literal)
	tiles := make([]gosx.Node, 0, len(names))
	for i, name := range names {
		dot := elWith("span", gosx.Attrs(gosx.Attr("class", "ldot "+dotPalette[i%len(dotPalette)])))
		children := []gosx.Node{dot, elWith("span", gosx.Attrs(gosx.Attr("class", "lname")), gosx.Text(name))}
		if langTokenSourceSet[name] {
			children = append(children, elWith("span", gosx.Attrs(gosx.Attr("class", "lts")), gosx.Text("TS")))
		}
		tiles = append(tiles, elWith("div", gosx.Attrs(gosx.Attr("class", "langtile")), children...))
	}
	return elWith("div", gosx.Attrs(gosx.Attr("class", "langgrid")), tiles...)
}

// ---- admonitions / callouts ------------------------------------------

// admonitionVariant maps mdpp's 5 GitHub-style admonition types
// (`> [!NOTE]` / `[!TIP]` / `[!IMPORTANT]` / `[!WARNING]` / `[!CAUTION]`,
// parsed by mdpp itself — see extensions.go's processAdmonitions) onto
// public/docs.css's 3 `.note` accent variants. tip keeps its name; important
// reads as "new" (design's yellow, attention-worthy accent — see e.g. "Why
// it's fast" / "Editor-grade" in the design source); warning and caution
// both read as "win" (design's pink gotcha/heads-up accent — see "Heads
// up" / "Contract that differs from C"). Plain [!NOTE] gets no variant
// class, matching the base `.note` look public/docs.css defines for it.
var admonitionVariant = map[string]string{
	"tip":       "note tip",
	"important": "note new",
	"warning":   "note win",
	"caution":   "note win",
}

var admonitionDefaultTag = map[string]string{
	"note":      "Note",
	"tip":       "Tip",
	"important": "Important",
	"warning":   "Warning",
	"caution":   "Caution",
}

// renderAdmonition renders `.note`. Content authoring convention: the
// callout's `.tag` label is whatever follows the marker on the same line —
// `> [!TIP] Good to know` → tag "Good to know" (mdpp parses this into
// Attrs["title"] itself); a bare `> [!TIP]` falls back to the type's default
// tag ("Tip").
func renderAdmonition(n *mdpp.Node, src []byte) gosx.Node {
	typ := strings.ToLower(strings.TrimSpace(n.Attrs["type"]))
	class := "note"
	if v := admonitionVariant[typ]; v != "" {
		class = v
	}
	tag := strings.TrimSpace(n.Attrs["title"])
	if tag == "" {
		tag = admonitionDefaultTag[typ]
		if tag == "" {
			tag = "Note"
		}
	}

	content := []gosx.Node{elWith("span", gosx.Attrs(gosx.Attr("class", "tag")), gosx.Text(tag))}
	children := n.Children
	if len(children) > 0 && children[0].Type == mdpp.NodeParagraph {
		// Matches design's `<p style="margin:0" class="p">` — the note box
		// already carries its own padding, so the lead paragraph shouldn't
		// double it up with `.p`'s normal top/bottom margin.
		content = append(content, elWith("p",
			gosx.Attrs(gosx.Attr("class", "p"), gosx.Attr("style", "margin:0")),
			renderInline(children[0].Children)...,
		))
		children = children[1:]
	}
	content = append(content, renderBlocks(children, src)...)

	return elWith("div", gosx.Attrs(gosx.Attr("class", class)), content...)
}

// renderPlainQuote handles a bare `> quote` blockquote that mdpp did not
// recognize as a `[!TYPE]` admonition (none exist in content/docs/*.md
// today; kept for robustness against future prose). Rendered as a neutral
// note box rather than dropped or left unstyled.
func renderPlainQuote(n *mdpp.Node, src []byte) gosx.Node {
	content := []gosx.Node{elWith("span", gosx.Attrs(gosx.Attr("class", "tag")), gosx.Text("Note"))}
	content = append(content, renderBlocks(n.Children, src)...)
	return elWith("div", gosx.Attrs(gosx.Attr("class", "note")), content...)
}

// ---- decide pairs (external-scanners.md "do you need a scanner?") ----

// tryDecidePair looks for two adjacent H3 sections at nodes[i:] whose
// headings start with "✗ " and "✓ " (the convention this pass adds to
// external-scanners.md's "You do NOT need a scanner for" / "You DO need a
// scanner for" headings, matching design's own literal heading text) and
// collapses them into design's `.decide` two-card comparison. Returns
// consumed=0 when the pattern doesn't match at i, leaving nodes[i] to render
// normally.
func tryDecidePair(nodes []*mdpp.Node, i int, src []byte) (int, gosx.Node) {
	n := nodes[i]
	if n == nil || n.Type != mdpp.NodeHeading || n.Level() != 3 {
		return 0, gosx.Node{}
	}
	title1 := strings.TrimSpace(n.Text())
	if !strings.HasPrefix(title1, "✗") {
		return 0, gosx.Node{}
	}

	j := i + 1
	var body1 []*mdpp.Node
	for j < len(nodes) && !(nodes[j].Type == mdpp.NodeHeading && nodes[j].Level() <= 3) {
		body1 = append(body1, nodes[j])
		j++
	}
	if j >= len(nodes) || nodes[j].Type != mdpp.NodeHeading || nodes[j].Level() != 3 {
		return 0, gosx.Node{}
	}
	title2 := strings.TrimSpace(nodes[j].Text())
	if !strings.HasPrefix(title2, "✓") {
		return 0, gosx.Node{}
	}

	k := j + 1
	var body2 []*mdpp.Node
	for k < len(nodes) && !(nodes[k].Type == mdpp.NodeHeading && nodes[k].Level() <= 3) {
		body2 = append(body2, nodes[k])
		k++
	}

	card1 := elWith("div", gosx.Attrs(gosx.Attr("class", "dc")),
		append([]gosx.Node{
			elWith("div", gosx.Attrs(gosx.Attr("class", "dh"), gosx.Attr("style", "color:var(--paper);background:var(--ink)")), gosx.Text(title1)),
		}, renderDecideBody(body1, src)...)...,
	)
	card2 := elWith("div", gosx.Attrs(gosx.Attr("class", "dc")),
		append([]gosx.Node{
			elWith("div", gosx.Attrs(gosx.Attr("class", "dh c-green")), gosx.Text(title2)),
		}, renderDecideBody(body2, src)...)...,
	)
	return k - i, elWith("div", gosx.Attrs(gosx.Attr("class", "decide")), card1, card2)
}

// renderDecideBody renders every block in a decide-pair section body,
// same as renderBlocks, except a plain bullet list renders as a bare <ul>
// (design scopes list styling to `.decide ul`/`.decide li` directly, not
// the `.list` class) instead of going through renderList's `ul.list`/
// feature-card/checklist dispatch. Both external-scanners.md sections carry
// more than just their list — a lead-in paragraph, a closing paragraph, or
// both — so every other block still renders normally rather than being
// dropped.
func renderDecideBody(body []*mdpp.Node, src []byte) []gosx.Node {
	out := make([]gosx.Node, 0, len(body))
	for _, n := range body {
		if n != nil && n.Type == mdpp.NodeList && !isOrderedList(n) && !isTaskList(n) {
			items := make([]gosx.Node, 0, len(n.Children))
			for _, li := range n.Children {
				items = append(items, elWith("li", nil, renderListItemChildren(li, src)...))
			}
			out = append(out, elWith("ul", nil, items...))
			continue
		}
		out = append(out, renderBlock(n, src)...)
	}
	return out
}

func isOrderedList(n *mdpp.Node) bool {
	return n.Attrs != nil && n.Attrs["ordered"] == "true"
}

// ---- lists -------------------------------------------------------------

// renderList dispatches a markdown list to whichever design component its
// shape matches:
//   - every item is `[Title](url) — description` → `.grid3` of `.feat`
//     cards (the design's "Where to next?" pattern; auto-detected, no
//     markdown convention needed beyond the links already being links).
//   - GFM task list (`- [ ] ...`) → `.checklist`.
//   - ordered list (`1. ...`) → a `.step`/`.stepn`/`.sc` sequence, with a
//     leading **bold** run in an item promoted to that step's `<h4>` title.
//   - anything else → the design's plain bulleted `ul.list`.
func renderList(n *mdpp.Node, src []byte) gosx.Node {
	if isFeatureLinkList(n) {
		return renderFeatureCards(n)
	}
	if isTaskList(n) {
		return renderChecklist(n, src)
	}
	if isOrderedList(n) {
		return renderStepList(n, src)
	}

	items := make([]gosx.Node, 0, len(n.Children))
	for _, li := range n.Children {
		items = append(items, elWith("li", nil, renderListItemChildren(li, src)...))
	}
	return elWith("ul", gosx.Attrs(gosx.Attr("class", "list")), items...)
}

// renderListItemChildren renders one list item's content. A "tight" item
// (its only child is a single paragraph) inlines straight into the <li>,
// matching the design's plain single-line list items instead of nesting a
// margined <p> inside every <li>. A "loose" item (multiple blocks — a
// paragraph plus a nested list or code block, for instance) renders each
// block normally.
func renderListItemChildren(li *mdpp.Node, src []byte) []gosx.Node {
	if len(li.Children) == 1 && li.Children[0].Type == mdpp.NodeParagraph {
		return renderInline(li.Children[0].Children)
	}
	return renderBlocks(li.Children, src)
}

func isTaskList(n *mdpp.Node) bool {
	if len(n.Children) == 0 {
		return false
	}
	for _, li := range n.Children {
		if li.Type != mdpp.NodeTaskListItem {
			return false
		}
	}
	return true
}

func renderChecklist(n *mdpp.Node, src []byte) gosx.Node {
	items := make([]gosx.Node, 0, len(n.Children))
	for _, li := range n.Children {
		items = append(items, elWith("li", nil, renderListItemChildren(li, src)...))
	}
	return elWith("ul", gosx.Attrs(gosx.Attr("class", "checklist")), items...)
}

// isFeatureLinkList reports whether every item in an unordered list opens
// with a link as its first inline content — the shape every "Next steps"
// closing section in content/docs/*.md already has
// (`- [Title](/docs/slug) — description`).
func isFeatureLinkList(n *mdpp.Node) bool {
	if n == nil || n.Type != mdpp.NodeList {
		return false
	}
	if isOrderedList(n) {
		return false
	}
	if len(n.Children) < 2 {
		return false
	}
	for _, li := range n.Children {
		if li.Type != mdpp.NodeListItem || len(li.Children) == 0 {
			return false
		}
		first := li.Children[0]
		if first.Type == mdpp.NodeParagraph {
			if len(first.Children) == 0 || first.Children[0].Type != mdpp.NodeLink {
				return false
			}
			continue
		}
		if first.Type != mdpp.NodeLink {
			return false
		}
	}
	return true
}

// renderFeatureCards renders design's `.grid3` of `.feat` cards. Each card
// is the design's border/shadow box wrapped directly in an `<a>` (design's
// own preview used a non-link `<div onClick>`, since this component backs
// real page navigation here — same adaptation app/layout.gsx already makes
// for `.navitem`).
func renderFeatureCards(n *mdpp.Node) gosx.Node {
	cards := make([]gosx.Node, 0, len(n.Children))
	for i, li := range n.Children {
		content := li.Children
		if len(content) > 0 && content[0].Type == mdpp.NodeParagraph {
			content = content[0].Children
		}
		if len(content) == 0 || content[0].Type != mdpp.NodeLink {
			continue
		}
		link := content[0]
		href := link.Attrs["href"]
		title := link.Text()
		desc := trimLeadingDash(content[1:])

		card := elWith("a",
			gosx.Attrs(gosx.Attr("class", "feat"), gosx.Attr("href", href)),
			elWith("div", gosx.Attrs(gosx.Attr("class", "featicon "+dotPalette[i%len(dotPalette)])), gosx.Text("→")),
			elWith("h4", nil, gosx.Text(title)),
			elWith("p", nil, renderInline(desc)...),
		)
		cards = append(cards, card)
	}
	return elWith("div", gosx.Attrs(gosx.Attr("class", "grid3")), cards...)
}

// trimLeadingDash strips the " — " / " - " separator content/docs/*.md uses
// between a next-steps link and its description, so the .feat card's <p>
// doesn't start with a stray dash.
func trimLeadingDash(nodes []*mdpp.Node) []*mdpp.Node {
	if len(nodes) == 0 || nodes[0].Type != mdpp.NodeText {
		return nodes
	}
	text := strings.TrimLeft(nodes[0].Literal, " ")
	text = strings.TrimPrefix(text, "—")
	text = strings.TrimPrefix(text, "-")
	text = strings.TrimLeft(text, " ")
	if text == nodes[0].Literal {
		return nodes
	}
	clone := *nodes[0]
	clone.Literal = text
	out := make([]*mdpp.Node, len(nodes))
	out[0] = &clone
	copy(out[1:], nodes[1:])
	return out
}

// ---- steps (.step/.stepn/.sc) ------------------------------------------

// renderStepList renders every item of an ordered list as a numbered
// `.step`. Ordered-list markdown syntax *is* the "step" authoring
// convention this pass adds — no invented pseudo-syntax, just giving
// standard `1. **Title.** body` lists (recovery-and-correctness.md,
// architecture.md, incremental-parsing.md, contributing.md,
// authoring-languages.md all already have one) the design's numbered-block
// treatment.
func renderStepList(n *mdpp.Node, src []byte) gosx.Node {
	steps := make([]gosx.Node, 0, len(n.Children))
	for i, li := range n.Children {
		title, body := splitBoldLead(li)
		steps = append(steps, renderStep(i+1, title, body, src))
	}
	return gosx.Fragment(steps...)
}

// splitBoldLead detects `<li><p><strong>Title.</strong> body...</p></li>`
// and pulls the bold run out as a step title, leaving the rest of the
// paragraph (plus any further blocks in a loose item) as the step body. An
// item with no bold lead (contributing.md's and incremental-parsing.md's
// ordered lists are plain text) returns an empty title and its content
// unchanged — renderStep skips the <h4> in that case.
func splitBoldLead(li *mdpp.Node) (string, []*mdpp.Node) {
	if len(li.Children) == 0 || li.Children[0].Type != mdpp.NodeParagraph {
		return "", li.Children
	}
	para := li.Children[0]
	if len(para.Children) == 0 || para.Children[0].Type != mdpp.NodeStrong {
		return "", li.Children
	}
	title := para.Children[0].Text()
	rest := &mdpp.Node{Type: mdpp.NodeParagraph, Children: trimLeadingSpace(para.Children[1:])}
	body := append([]*mdpp.Node{rest}, li.Children[1:]...)
	return title, body
}

func renderStep(n int, title string, body []*mdpp.Node, src []byte) gosx.Node {
	color := dotPalette[(n-1)%len(dotPalette)]
	sc := make([]gosx.Node, 0, len(body)+1)
	if strings.TrimSpace(title) != "" {
		sc = append(sc, elWith("h4", nil, gosx.Text(title)))
	}
	sc = append(sc, renderBlocks(body, src)...)

	return elWith("div", gosx.Attrs(gosx.Attr("class", "step")),
		elWith("div", gosx.Attrs(gosx.Attr("class", "stepn "+color)), gosx.Text(strconv.Itoa(n))),
		elWith("div", gosx.Attrs(gosx.Attr("class", "sc")), sc...),
	)
}

// ---- tables --------------------------------------------------------------

// renderTable dispatches to the design's `.statstrip` for a small
// key/value stats table (see isStatTable) or the general `.tbl` grid
// otherwise.
func renderTable(n *mdpp.Node) gosx.Node {
	if isStatTable(n) {
		return renderStatTable(n)
	}
	if len(n.Children) == 0 {
		return gosx.Text("")
	}
	header := n.Children[0]
	rows := n.Children[1:]
	aligns := strings.Split(n.Attrs["align"], ",")
	gridStyle := gridTemplate(columnWidths(header, rows))

	headerCells := make([]gosx.Node, 0, len(header.Children))
	for _, c := range header.Children {
		headerCells = append(headerCells, elWith("span", gosx.Attrs(gosx.Attr("class", "td")), renderInline(c.Children)...))
	}
	trh := elWith("div", gosx.Attrs(gosx.Attr("class", "trh"), gosx.Attr("style", gridStyle)), headerCells...)

	body := make([]gosx.Node, 0, len(rows))
	for _, row := range rows {
		if row.Type != mdpp.NodeTableRow {
			continue
		}
		cells := make([]gosx.Node, 0, len(row.Children))
		for ci, c := range row.Children {
			class := "td"
			if ci == 0 && isKeyCell(c) {
				class += " k"
			}
			pairs := []any{gosx.Attr("class", class)}
			if ci < len(aligns) && aligns[ci] != "" {
				pairs = append(pairs, gosx.Attr("style", "text-align:"+aligns[ci]))
			}
			cells = append(cells, elWith("span", gosx.Attrs(pairs...), renderInline(c.Children)...))
		}
		body = append(body, elWith("div", gosx.Attrs(gosx.Attr("class", "tr"), gosx.Attr("style", gridStyle)), cells...))
	}

	rowsNodes := append([]gosx.Node{trh}, body...)
	return elWith("div", gosx.Attrs(gosx.Attr("class", "tbl")), rowsNodes...)
}

// isKeyCell reports whether a body cell's content is inline code — the
// signal content/docs/*.md's own tables already use to distinguish a
// "symbol" column (queries.md's predicate table, external-scanners.md's
// lexer-method table) from an ordinary text column. Mirrors design's own
// `.td.k` usage, which keys exactly these symbol-shaped first columns and
// nothing else.
func isKeyCell(cell *mdpp.Node) bool {
	for _, c := range cell.Children {
		if c.Type == mdpp.NodeText && strings.TrimSpace(c.Literal) == "" {
			continue
		}
		return c.Type == mdpp.NodeCodeSpan
	}
	return false
}

// columnWidths derives a content-weighted fr-width per column from the
// widest cell (header or body) in that column, floored at 4 characters.
// Markdown tables have no per-instance column authoring, so this
// derives an equivalent automatically from the actual cell text.
func columnWidths(header *mdpp.Node, rows []*mdpp.Node) []float64 {
	n := len(header.Children)
	widths := make([]float64, n)
	for i, c := range header.Children {
		// .trh renders header text uppercase with letter-spacing (see
		// public/docs.css's `.trh{text-transform:uppercase;letter-spacing:.6px}`),
		// which needs visibly more room per character than the same rune
		// count in ordinary body text — weight it up so a short-but-wide
		// header (e.g. "Language") doesn't get squeezed into wrapping by a
		// column whose data rows just happen to be short strings.
		widths[i] = float64(len([]rune(strings.TrimSpace(c.Text())))) * 1.3
	}
	for _, row := range rows {
		if row.Type != mdpp.NodeTableRow {
			continue
		}
		for i, c := range row.Children {
			if i >= n {
				continue
			}
			if l := float64(len([]rune(strings.TrimSpace(c.Text())))); l > widths[i] {
				widths[i] = l
			}
		}
	}
	for i := range widths {
		if widths[i] < 4 {
			widths[i] = 4
		}
	}
	return widths
}

func gridTemplate(widths []float64) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strconv.FormatFloat(w, 'f', 1, 64) + "fr"
	}
	return "grid-template-columns:" + strings.Join(parts, " ")
}

// isStatTable recognizes languages.md's "The full catalog" scan table — a
// 2-column table with a blank `| | |` header row — as the content-as-data
// shape design's `.statstrip` was built for (a small, static set of
// value+label pairs), rather than a reference table.
func isStatTable(n *mdpp.Node) bool {
	if len(n.Children) < 2 {
		return false
	}
	header := n.Children[0]
	if header.Type != mdpp.NodeTableRow || len(header.Children) != 2 {
		return false
	}
	for _, c := range header.Children {
		if strings.TrimSpace(c.Text()) != "" {
			return false
		}
	}
	return true
}

func renderStatTable(n *mdpp.Node) gosx.Node {
	stats := make([]gosx.Node, 0, len(n.Children)-1)
	for i, row := range n.Children {
		if i == 0 || row.Type != mdpp.NodeTableRow || len(row.Children) < 2 {
			continue
		}
		label := strings.TrimSpace(row.Children[0].Text())
		value := strings.TrimSpace(row.Children[1].Text())
		stats = append(stats, elWith("div", gosx.Attrs(gosx.Attr("class", "stat")),
			elWith("div", gosx.Attrs(gosx.Attr("class", "sv "+tPalette[(len(stats))%len(tPalette)])), gosx.Text(value)),
			elWith("div", gosx.Attrs(gosx.Attr("class", "sl")), gosx.Text(label)),
		))
	}
	// design's own .statstrip is a fixed 4-column grid sized to its 4-stat
	// mockup examples; content/docs/languages.md's real scan table has 6
	// rows, so this instance uses an auto-fit grid (same minmax idiom
	// public/docs.css already uses for .langgrid) instead of forcing a 5th/6th
	// stat to overflow a hardcoded 4-column row.
	return elWith("div",
		gosx.Attrs(gosx.Attr("class", "statstrip"), gosx.Attr("style", "grid-template-columns:repeat(auto-fit,minmax(150px,1fr))")),
		stats...,
	)
}
