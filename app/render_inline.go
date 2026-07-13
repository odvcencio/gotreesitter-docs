package docs

import (
	"strings"

	"github.com/odvcencio/mdpp"
	"m31labs.dev/gosx"
)

// renderInline renders a run of inline mdpp nodes (the children of a
// paragraph, heading, list-item text, table cell, etc.) into gosx children.
func renderInline(nodes []*mdpp.Node) []gosx.Node {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]gosx.Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, renderInlineNode(n))
	}
	return out
}

func renderInlineNode(n *mdpp.Node) gosx.Node {
	if n == nil {
		return gosx.Text("")
	}
	switch n.Type {
	case mdpp.NodeText:
		return gosx.Text(n.Literal)
	case mdpp.NodeSoftBreak:
		// A wrapped line inside one source paragraph — not a real line
		// break in the rendered design (prose flows in normal block
		// context, so HTML collapses this to a single space visually).
		return gosx.Text("\n")
	case mdpp.NodeHardBreak:
		return gosx.El("br")
	case mdpp.NodeEmphasis:
		return elWith("em", nil, renderInline(n.Children)...)
	case mdpp.NodeStrong:
		return elWith("strong", nil, renderInline(n.Children)...)
	case mdpp.NodeStrikethrough:
		return elWith("del", nil, renderInline(n.Children)...)
	case mdpp.NodeCodeSpan:
		// The site's inline-code convention uses the mono token in running
		// prose rather than a boxed code pill.
		return elWith("code", gosx.Attrs(gosx.Attr("class", "mono")), gosx.Text(n.Literal))
	case mdpp.NodeLink:
		return renderLink(n)
	case mdpp.NodeImage:
		return renderImage(n)
	case mdpp.NodeHTMLInline:
		return gosx.RawHTML(n.Literal)
	case mdpp.NodeSuperscript:
		return elWith("sup", nil, gosx.Text(n.Literal))
	case mdpp.NodeSubscript:
		return elWith("sub", nil, gosx.Text(n.Literal))
	default:
		if len(n.Children) > 0 {
			return gosx.Fragment(renderInline(n.Children)...)
		}
		return gosx.Text(n.Literal)
	}
}

func renderLink(n *mdpp.Node) gosx.Node {
	href := n.Attrs["href"]
	if href == "" {
		if raw := n.Attrs["raw"]; raw != "" {
			return gosx.Text(raw)
		}
		return gosx.Fragment(renderInline(n.Children)...)
	}
	pairs := []any{gosx.Attr("href", href)}
	if title := n.Attrs["title"]; title != "" {
		pairs = append(pairs, gosx.Attr("title", title))
	}
	switch {
	case strings.HasPrefix(href, "http://"), strings.HasPrefix(href, "https://"):
		pairs = append(pairs, gosx.Attr("target", "_blank"), gosx.Attr("rel", "noopener noreferrer"))
	}
	return elWith("a", gosx.Attrs(pairs...), renderInline(n.Children)...)
}

func renderImage(n *mdpp.Node) gosx.Node {
	attrs := gosx.Attrs(gosx.Attr("src", n.Attrs["src"]), gosx.Attr("alt", n.Attrs["alt"]))
	return elWith("img", attrs)
}
