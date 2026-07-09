package docs

import (
	"strings"

	"github.com/odvcencio/mdpp"
)

// normalizeBlocks repairs a real mdpp parsing quirk found while wiring this
// renderer up against content/docs/*.md's actual (hard-wrapped, ~100-column)
// prose: a list item whose text wraps onto a second physical source line
// sometimes parses as *two* sibling Paragraph children instead of one
// Paragraph with a SoftBreak in the middle — and for a list's last item, the
// wrapped continuation line can escape the list entirely, landing as a
// *sibling* Paragraph immediately after the list closes. Both shapes were
// confirmed against mdpp v0.2.5 directly (not a hypothesis): the same list
// markdown parses correctly in isolation but splits once enough preceding
// document content is present, so this is systemic to real content/docs/*.md
// pages, not fixable by reformatting one file's indentation.
//
// This walks a block-node slice once, never mutating the shared,
// process-lifetime doc.Parsed tree (content/docs/*.md loads once at package
// init — see content.go — and every request renders from the same cached
// *mdpp.Document), and returns a repaired clone: multi-paragraph list items
// merge back into one, and an escaped trailing continuation paragraph
// reattaches to the list item it belongs to.
func normalizeBlocks(nodes []*mdpp.Node) []*mdpp.Node {
	out := make([]*mdpp.Node, 0, len(nodes))
	for i := 0; i < len(nodes); i++ {
		n := nodes[i]
		if n == nil {
			continue
		}
		if n.Type == mdpp.NodeList {
			list := normalizeList(n)
			if i+1 < len(nodes) && isOrphanedContinuation(nodes[i+1]) {
				list = attachOrphanContinuation(list, nodes[i+1])
				i++ // consumed as part of the list
			}
			out = append(out, list)
			continue
		}
		if n.Type == mdpp.NodeAdmonition || n.Type == mdpp.NodeBlockquote {
			// A second, related mdpp quirk: inside a `> [!TYPE] ...` block
			// spanning 3+ physical `>`-prefixed source lines, mdpp correctly
			// strips the `>` marker from the first two lines but leaves a
			// literal "> " (or ">") sitting on the front of the third-and-later
			// continuation lines' Text node — confirmed directly against
			// getting-started.md's multi-line [!NOTE]. Same root shape as the
			// list-continuation quirk above (a per-line structural prefix mdpp
			// fails to consume past the second physical line), same fix
			// strategy: strip it in a post-parse pass rather than fight the
			// upstream parser.
			clone := *n
			clone.Children = normalizeBlocks(stripQuoteLineLeak(n.Children))
			out = append(out, &clone)
			continue
		}
		if len(n.Children) > 0 {
			clone := *n
			clone.Children = normalizeBlocks(n.Children)
			out = append(out, &clone)
			continue
		}
		out = append(out, n)
	}
	return out
}

// normalizeList clones a list, merging each item's wrapped-line paragraphs
// back into one and recursing into each item's own children (so a list
// nested inside a list item, or inside a loose item's extra blocks, gets
// the same repair).
func normalizeList(n *mdpp.Node) *mdpp.Node {
	items := make([]*mdpp.Node, len(n.Children))
	for i, li := range n.Children {
		clone := *li
		clone.Children = normalizeBlocks(mergeParagraphRuns(li.Children))
		items[i] = &clone
	}
	listClone := *n
	listClone.Children = items
	return &listClone
}

// mergeParagraphRuns collapses consecutive Paragraph children into one,
// joining them with a synthetic SoftBreak the same way the (correctly
// parsed, in isolation) source markdown would have — a wrapped list item is
// one logical paragraph in the source, regardless of how many Paragraph
// nodes mdpp produced for it here. Non-Paragraph children (a nested list, a
// code block in a loose item) pass through untouched and end any run.
func mergeParagraphRuns(children []*mdpp.Node) []*mdpp.Node {
	out := make([]*mdpp.Node, 0, len(children))
	var run []*mdpp.Node
	inRun := false
	flush := func() {
		if inRun {
			out = append(out, &mdpp.Node{Type: mdpp.NodeParagraph, Children: run})
			run = nil
			inRun = false
		}
	}
	for _, c := range children {
		if c == nil {
			continue
		}
		if c.Type != mdpp.NodeParagraph {
			flush()
			out = append(out, c)
			continue
		}
		if inRun {
			run = append(run, &mdpp.Node{Type: mdpp.NodeSoftBreak})
		}
		run = append(run, trimLeadingSpace(c.Children)...)
		inRun = true
	}
	flush()
	return out
}

// trimLeadingSpace strips the raw source indentation that bleeds through
// literally onto a continuation line's first Text node in the split shape
// this file works around (mdpp does not re-strip it since, from its own
// AST's point of view, that text is starting a fresh paragraph).
func trimLeadingSpace(nodes []*mdpp.Node) []*mdpp.Node {
	if len(nodes) == 0 || nodes[0].Type != mdpp.NodeText {
		return nodes
	}
	trimmed := strings.TrimLeft(nodes[0].Literal, " \t")
	if trimmed == nodes[0].Literal {
		return nodes
	}
	if trimmed == "" {
		return nodes[1:]
	}
	clone := *nodes[0]
	clone.Literal = trimmed
	out := make([]*mdpp.Node, len(nodes))
	out[0] = &clone
	copy(out[1:], nodes[1:])
	return out
}

// isOrphanedContinuation recognizes the escaped-last-line shape: a
// Paragraph whose very first Text child still carries literal leading
// whitespace is, in every observed case, a wrapped list-item continuation
// line that mdpp placed outside its list rather than a genuine new
// paragraph (real prose never starts a paragraph with a literal space).
func isOrphanedContinuation(n *mdpp.Node) bool {
	if n == nil || n.Type != mdpp.NodeParagraph || len(n.Children) == 0 {
		return false
	}
	first := n.Children[0]
	return first.Type == mdpp.NodeText && strings.HasPrefix(first.Literal, " ")
}

// attachOrphanContinuation reattaches an escaped continuation paragraph to
// the last item of the list it fell out of, merging it in exactly like a
// same-item wrapped paragraph (see mergeParagraphRuns).
func attachOrphanContinuation(list *mdpp.Node, orphan *mdpp.Node) *mdpp.Node {
	if len(list.Children) == 0 {
		return list
	}
	items := append([]*mdpp.Node(nil), list.Children...)
	last := len(items) - 1
	lastItem := *items[last]
	lastItem.Children = mergeParagraphRuns(append(append([]*mdpp.Node(nil), lastItem.Children...), orphan))
	items[last] = &lastItem
	listClone := *list
	listClone.Children = items
	return &listClone
}

// stripQuoteLineLeak recursively strips a leaked literal "> " (or ">")
// prefix — mdpp's third-and-later-line blockquote/admonition continuation
// quirk — from a `> [!TYPE]`/`>` block. A real ">" would never legitimately
// open a line inside blockquote prose, so this is safe to apply
// unconditionally within that subtree.
//
// The leak shows up in two AST shapes: as a whole extra Text sibling
// immediately after a SoftBreak (direct children of a Paragraph), or
// embedded mid-string as "...\n> ..." inside a single Text node whose line
// wrap fell inside an inline span like a Link — confirmed against both
// directly (see performance.md's closing [!IMPORTANT] callout, whose link
// text itself wraps across a source line). mapTextLiterals's single
// full-depth pass handles the embedded-mid-string shape everywhere in the
// subtree (including inside Link/Strong/etc.) before the sibling-structural
// pass handles the SoftBreak-then-Text shape.
func stripQuoteLineLeak(nodes []*mdpp.Node) []*mdpp.Node {
	return stripQuoteLineLeakSiblings(mapTextLiterals(nodes, stripEmbeddedQuoteMarker))
}

func stripQuoteLineLeakSiblings(nodes []*mdpp.Node) []*mdpp.Node {
	out := make([]*mdpp.Node, len(nodes))
	for i, n := range nodes {
		switch {
		case n == nil:
			out[i] = n
		case n.Type == mdpp.NodeParagraph:
			clone := *n
			clone.Children = stripQuoteLineLeakInline(n.Children)
			out[i] = &clone
		case len(n.Children) > 0:
			clone := *n
			clone.Children = stripQuoteLineLeakSiblings(n.Children)
			out[i] = &clone
		default:
			out[i] = n
		}
	}
	return out
}

// mapTextLiterals clones nodes with fn applied to every Text node's Literal
// found at any depth, including inside inline wrappers (Link, Strong, Em,
// ...) that stripQuoteLineLeakSiblings' sibling-level walk can't see into.
func mapTextLiterals(nodes []*mdpp.Node, fn func(string) string) []*mdpp.Node {
	out := make([]*mdpp.Node, len(nodes))
	for i, n := range nodes {
		switch {
		case n == nil:
			out[i] = n
		case n.Type == mdpp.NodeText:
			if s := fn(n.Literal); s != n.Literal {
				clone := *n
				clone.Literal = s
				out[i] = &clone
			} else {
				out[i] = n
			}
		case len(n.Children) > 0:
			clone := *n
			clone.Children = mapTextLiterals(n.Children, fn)
			out[i] = &clone
		default:
			out[i] = n
		}
	}
	return out
}

// stripEmbeddedQuoteMarker collapses a leaked "\n> " / "\n>" sequence
// embedded mid-Text-node back to the single space a correctly-parsed
// soft-wrapped line would have rendered as.
func stripEmbeddedQuoteMarker(s string) string {
	s = strings.ReplaceAll(s, "\n> ", " ")
	s = strings.ReplaceAll(s, "\n>", " ")
	return s
}

func stripQuoteLineLeakInline(inline []*mdpp.Node) []*mdpp.Node {
	out := make([]*mdpp.Node, len(inline))
	afterBreak := false
	for i, c := range inline {
		if afterBreak && c != nil && c.Type == mdpp.NodeText {
			if trimmed, ok := cutQuoteMarker(c.Literal); ok {
				clone := *c
				clone.Literal = trimmed
				out[i] = &clone
				afterBreak = false
				continue
			}
		}
		out[i] = c
		afterBreak = c != nil && (c.Type == mdpp.NodeSoftBreak || c.Type == mdpp.NodeHardBreak)
	}
	return out
}

func cutQuoteMarker(s string) (string, bool) {
	if trimmed := strings.TrimPrefix(s, "> "); trimmed != s {
		return trimmed, true
	}
	if trimmed := strings.TrimPrefix(s, ">"); trimmed != s {
		return trimmed, true
	}
	return s, false
}
