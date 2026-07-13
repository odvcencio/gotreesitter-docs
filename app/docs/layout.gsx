package docs

// Layout is the nested /docs/* layout. The persistent shell (topbar +
// sidebar + main) lives once in the root app/layout.gsx so it wraps every
// route, landing page included. The site has a single shell rather than a
// docs-only duplicate. This nested layout is a
// pass-through on top of that; /docs/* pages render straight into the
// root's `.main`.
func Layout() Node {
	return <Slot />
}
