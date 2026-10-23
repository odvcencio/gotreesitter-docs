package docs

import "m31labs.dev/gosx"

var tag = "<script src=\"/x.js\"></script>"

var okTag = `<script type="application/json">{}</script>`

func node() gosx.Node {
	return gosx.El("script", nil)
}
