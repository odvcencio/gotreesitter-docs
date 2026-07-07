package docs

func Layout() Node {
	return <div class="docs-shell">
		<a href="#docs-main" class="skip-link">Skip to content</a>
		<header class="mobile-bar">
			<div class="mobile-bar-head">
				<div class="mobile-branding">
					<span class="mobile-kicker">GoTreeSitter Docs</span>
					<a href="/" data-gosx-link class="brand">GoTreeSitter</a>
				</div>
				<details class="route-drawer">
					<summary class="route-drawer-summary">
						<span class="route-drawer-backdrop" aria-hidden="true"></span>
						<span class="route-drawer-toggle">
							<span class="route-drawer-toggle-kicker">Routes</span>
							<span class="route-drawer-toggle-copy route-drawer-toggle-copy-open">Browse sections</span>
							<span class="route-drawer-toggle-copy route-drawer-toggle-copy-close">Close</span>
						</span>
					</summary>
					<div class="route-drawer-panel">
						<div class="brand-lockup route-drawer-lockup">
							<span class="eyebrow">GoTreeSitter Docs</span>
							<a href="/" data-gosx-link class="brand">GoTreeSitter</a>
							<p class="brand-copy">
								A pure-Go, byte-exact reimplementation of tree-sitter: incremental parsing, queries, and 206 grammars, with no CGo.
							</p>
						</div>
						<nav class="doc-nav">
							<DocsNavigation></DocsNavigation>
						</nav>
						<DocsShortcuts></DocsShortcuts>
					</div>
				</details>
			</div>
		</header>
		<aside class="sidebar">
			<div class="sidebar-frame">
				<div class="brand-lockup">
					<span class="eyebrow">GoTreeSitter Docs</span>
					<a href="/" data-gosx-link class="brand">GoTreeSitter</a>
					<p class="brand-copy">
						A pure-Go, byte-exact reimplementation of tree-sitter: incremental parsing, queries, and 206 grammars, with no CGo.
					</p>
				</div>
				<nav class="doc-nav">
					<DocsNavigation></DocsNavigation>
				</nav>
				<DocsShortcuts></DocsShortcuts>
			</div>
		</aside>
		<main class="main" id="docs-main">
			<Slot />
			<footer class="page-footer">
				GoTreeSitter docs: parsing, incremental re-parse, queries, language grammars, and the internals that keep output byte-exact with C tree-sitter.
			</footer>
		</main>
	</div>
}

func DocsNavLink(props any) Node {
	return <>
		<If when={props.Active}>
			<a href={props.Href} data-gosx-link class="nav-link active">{props.Label}</a>
		</If>
		<If when={props.Active == false}>
			<a href={props.Href} data-gosx-link class="nav-link">{props.Label}</a>
		</If>
	</>
}

// DocsNavigation renders the grouped sidebar nav from navGroups, which is
// bound into the render environment per-request by the app/layout.gsx file
// module (see app/layout.server.go). navGroups is built from the loaded
// content/docs collection's frontmatter (nav_group, order) — it is not
// hand-maintained here, so a new content/docs/*.md file appears
// automatically once it carries a recognized nav_group.
func DocsNavigation() Node {
	return <>
		<Each as="group" of={navGroups}>
			<div class="nav-group">
				<span class="nav-group-title">{group.name}</span>
				<div class="nav-group-links">
					<Each as="item" of={group.items}>
						<DocsNavLink
							href={"/docs/" + item.slug}
							label={item.label}
							active={request.path == "/docs/" + item.slug}
						></DocsNavLink>
					</Each>
				</div>
			</div>
		</Each>
	</>
}

func DocsShortcuts() Node {
	return <div class="sidebar-foot">
		<span class="foot-label">Start here</span>
		<div class="shortcut-grid">
			<a href="/docs/introduction" data-gosx-link class="chip">Introduction</a>
			<a href="/docs/getting-started" data-gosx-link class="chip">Getting started</a>
			<a href="/docs/queries" data-gosx-link class="chip">Queries</a>
		</div>
	</div>
}
