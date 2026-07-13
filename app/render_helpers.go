package docs

import "m31labs.dev/gosx"

// elWith builds a gosx element from a tag, an attribute list (nil is fine),
// and a slice of children — gosx.El itself only accepts children as
// individual variadic `any` args, so this is the adapter every renderer in
// this Phase B pipeline (render.go, render_blocks.go, render_inline.go)
// funnels through instead of hand-spreading slices at each call site.
func elWith(tag string, attrs gosx.AttrList, children ...gosx.Node) gosx.Node {
	args := make([]any, 0, len(children)+1)
	args = append(args, attrs)
	for _, c := range children {
		args = append(args, c)
	}
	return gosx.El(tag, args...)
}

// headingClass collapses gotreesitter-docs markdown heading levels onto the
// site's two visual heading tokens: public/docs.css defines `.h2` (with a
// following `.underbar`) and `.h3` — there is no visual `.h4`+, and the content
// pages don't use one (verified against content/docs/*.md).
func headingClass(level int) string {
	if level <= 2 {
		return "h2"
	}
	return "h3"
}

// headingTag keeps the real semantic HTML heading level even though the
// visual size is clamped to h2/h3 by headingClass, so screen-reader outline
// navigation still reflects the document's actual structure.
func headingTag(level int) string {
	switch {
	case level <= 1:
		return "h1"
	case level == 2:
		return "h2"
	case level == 3:
		return "h3"
	case level == 4:
		return "h4"
	case level == 5:
		return "h5"
	default:
		return "h6"
	}
}
