// Package docs (this file's declared package; import path
// github.com/odvcencio/gotreesitter-docs/app — see content.go/docs_nav.go
// for the rest of this pipeline) is Phase B of the neo-brutalist design
// pass: it renders content/docs/*.md through the design's own components
// (design/GoTreeSitter-Docs.html, design/design.css) instead of the plain
// `<article class="prose">` Phase A shipped.
//
// RenderDesignDoc replaces the content.Library's default MDPP-to-HTML
// renderer with a hand-written one that walks the mdpp AST (doc.Parsed)
// directly and builds gosx.Node trees carrying the design's classes —
// .h2+.underbar, .code/.codehead/.codebody with tk-* highlight spans,
// .note callouts, .tbl tables, .step/.grid3/.checklist/.decide for the
// bespoke structures a couple of pages need. See render_blocks.go and
// render_inline.go for the node-by-node rendering, highlight.go for the
// gotreesitter-backed syntax highlighter.
package docs

import (
	"strings"

	"github.com/odvcencio/mdpp"
	"m31labs.dev/gosx"
	"m31labs.dev/gosx/content"
)

// RenderDesignDoc is wired into every content/docs/*.md page in
// app/docs/__slug/page.server.go (`doc.Render(docsapp.RenderDesignDoc)`,
// called from that file via its existing `docsapp` import alias for this
// package).
var RenderDesignDoc = content.RendererFunc(renderDesignDoc)

func renderDesignDoc(doc content.Document) (gosx.Node, error) {
	var children []*mdpp.Node
	var src []byte
	if doc.Parsed != nil && doc.Parsed.Root != nil {
		children = doc.Parsed.Root.Children
		src = doc.Parsed.Source
	}
	children = dropLeadingH1(children)
	children = normalizeBlocks(children)

	nodes := make([]gosx.Node, 0, len(children)+4)
	nodes = append(nodes, renderEyebrow(doc))
	nodes = append(nodes, elWith("h1", gosx.Attrs(gosx.Attr("class", "h1")), gosx.Text(pageTitle(doc))))
	if lead := pageLead(doc); lead != "" {
		nodes = append(nodes, elWith("p", gosx.Attrs(gosx.Attr("class", "lead")), gosx.Text(lead)))
	}

	if strings.TrimSpace(doc.Frontmatter["layout"]) == "steps" {
		nodes = append(nodes, renderStepsLayout(children, src)...)
	} else {
		nodes = append(nodes, renderBlocks(children, src)...)
	}

	return gosx.Fragment(nodes...), nil
}

// dropLeadingH1 skips a redundant top-of-body `# Title` heading (two
// content/docs/*.md files carried one before this pass; performance.md and
// recovery-and-correctness.md have since had theirs removed, but this stays
// defensive against a future page re-adding one) — RenderDesignDoc always
// synthesizes the page's real `.h1` from frontmatter (`title`), so a body H1
// would otherwise render as a second, design-less `<h1>`.
func dropLeadingH1(children []*mdpp.Node) []*mdpp.Node {
	if len(children) > 0 && children[0] != nil &&
		children[0].Type == mdpp.NodeHeading && children[0].Level() == 1 {
		return children[1:]
	}
	return children
}

func pageTitle(doc content.Document) string {
	if t := strings.TrimSpace(doc.Frontmatter["title"]); t != "" {
		return t
	}
	return doc.Slug
}

func pageLead(doc content.Document) string {
	return strings.TrimSpace(doc.Frontmatter["description"])
}

func renderEyebrow(doc content.Document) gosx.Node {
	return elWith("span", gosx.Attrs(gosx.Attr("class", "eyebrow")), gosx.Text(docsPageEyebrow(doc)))
}

// ---- steps-layout pages (getting-started.md) ---------------------------

// h2Section is one H2-bounded run of a document: the heading itself plus
// every block up to (not including) the next H2.
type h2Section struct {
	heading *mdpp.Node
	body    []*mdpp.Node
}

// splitH2Sections splits a flat block list into the content before the
// first H2 ("intro") and a sequence of H2-bounded sections.
func splitH2Sections(nodes []*mdpp.Node) (intro []*mdpp.Node, sections []h2Section) {
	i := 0
	for i < len(nodes) && !isH2(nodes[i]) {
		intro = append(intro, nodes[i])
		i++
	}
	for i < len(nodes) {
		heading := nodes[i]
		i++
		var body []*mdpp.Node
		for i < len(nodes) && !isH2(nodes[i]) {
			body = append(body, nodes[i])
			i++
		}
		sections = append(sections, h2Section{heading: heading, body: body})
	}
	return intro, sections
}

func isH2(n *mdpp.Node) bool {
	return n != nil && n.Type == mdpp.NodeHeading && n.Level() == 2
}

// renderStepsLayout is the `layout: steps` frontmatter opt-in (currently
// only getting-started.md): every H2 section becomes a numbered
// `.step`/`.stepn`/`.sc` block instead of a plain `.h2`+`.underbar` section,
// porting design/GoTreeSitter-Docs.html's getting-started page structure
// without inventing new markdown syntax — the section heading itself
// becomes the step's `<h4>` title.
//
// Once a section's entire body reduces to a single "next steps" link list,
// steps numbering stops: that section (design's own getting-started page
// ends the same way, with an un-numbered "Where to next?" `.grid3` of
// `.feat` cards, not a 5th/6th step) and everything after it renders
// normally instead.
func renderStepsLayout(nodes []*mdpp.Node, src []byte) []gosx.Node {
	intro, sections := splitH2Sections(nodes)
	out := renderBlocks(intro, src)

	stepIdx := 0
	tail := false
	for _, sec := range sections {
		if !tail && isFeatureLinkSection(sec.body) {
			tail = true
		}
		if tail {
			out = append(out, renderHeading(sec.heading)...)
			out = append(out, renderBlocks(sec.body, src)...)
			continue
		}
		stepIdx++
		title := ""
		if sec.heading != nil {
			title = sec.heading.Text()
		}
		out = append(out, renderStep(stepIdx, title, sec.body, src))
	}
	return out
}

// isFeatureLinkSection reports whether a section's body is (aside from
// nothing else) a single "next steps"-shaped link list.
func isFeatureLinkSection(body []*mdpp.Node) bool {
	var only *mdpp.Node
	for _, n := range body {
		if n == nil {
			continue
		}
		if only != nil {
			return false
		}
		only = n
	}
	return only != nil && isFeatureLinkList(only)
}
