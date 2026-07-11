package docs

// Layout is the site-wide root layout: it renders the design's persistent
// shell (design/GoTreeSitter-Docs.html's `.shell` > `.topbar` + `.body`
// > `.sidebar` + `.main`) around every route, including the landing page
// ("/") — the design has one shell for the whole site, not a docs-only one,
// and curl-ing "/" needs to show the topbar and sidebar alongside the hero.
// app/docs/layout.gsx (the nested /docs/* layout) is a pass-through on top
// of this.
func Layout() Node {
	return <div class="shell">
		<a href="#docs-main" class="skip-link">Skip to content</a>
		<header class="topbar">
			<a href="/" data-gosx-link class="brand">
				<span class="blk"><i></i><i></i><i></i><i></i></span>
				gotreesitter
			</a>
			<span class="ver mono">v0.23.1</span>
			<span class="tspacer"></span>
			<span class="status"><i></i>v0.23.1 · 206/206 structural parity</span>
			<a class="ghlink" href="https://github.com/odvcencio/gotreesitter" target="_blank">GitHub ↗</a>
		</header>
		<div class="body">
			<aside class="sidebar">
				<DocsNavigation></DocsNavigation>
			</aside>
			<main class="main" id="docs-main">
				<Slot />
			</main>
		</div>
	</div>
}

// DocsNavLink renders one sidebar `<li class="navitem">` entry, matching
// design.css's `.navitem`/`.ndot` look but with a real `<a href>` (the
// design's preview used a non-link `<li onClick=...>`, since it's a
// single-file SPA-style component preview; this site is file-routed).
func DocsNavLink(props any) Node {
	return <>
		<If when={props.Active}>
			<li class="navitem on">
				<a href={props.Href} data-gosx-link class="nav-anchor">
					<span class={"ndot " + props.Color}></span>
					{props.Label}
				</a>
			</li>
		</If>
		<If when={props.Active == false}>
			<li class="navitem">
				<a href={props.Href} data-gosx-link class="nav-anchor">
					<span class={"ndot " + props.Color}></span>
					{props.Label}
				</a>
			</li>
		</If>
	</>
}

// DocsNavigation renders the 5 grouped sidebar sections (`.navsec` +
// `.navlist`) from navGroups, which is bound into the render environment
// per-request by the app/layout.gsx file module (see app/layout.server.go).
// navGroups is built from the loaded content/docs collection's frontmatter
// (nav_group, order) in app/docs_nav.go — it is not hand-maintained here,
// so a new content/docs/*.md file appears automatically once it carries a
// recognized nav_group.
func DocsNavigation() Node {
	return <>
		<Each as="group" of={navGroups}>
			<div class="navsec">{group.name}</div>
			<ul class="navlist">
				<Each as="item" of={group.items}>
					<DocsNavLink
						href={"/docs/" + item.slug}
						label={item.label}
						color={item.color}
						active={request.path == "/docs/" + item.slug}
					></DocsNavLink>
				</Each>
			</ul>
		</Each>
	</>
}
