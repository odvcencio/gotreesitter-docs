package docs

// Layout is the nested /docs/* layout. The persistent shell (topbar +
// sidebar + main) lives once in the root app/layout.gsx so it wraps every
// route, landing page included — design/GoTreeSitter-Docs.html has a single
// shell for the whole site, not a docs-only one. This nested layout is a
// pass-through on top of that; /docs/* pages render straight into the
// root's `.main`.
func Layout() Node {
	return <Slot />
}
