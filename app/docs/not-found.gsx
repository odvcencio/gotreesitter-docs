package docs

func Page() Node {
	return <article class="prose">
		<div class="page-topper">
			<span class="eyebrow">Docs Missing</span>
			<p class="lede">
				This is the
				<span class="inline-code">/docs</span>
				scoped not-found page, not the site-wide fallback.
			</p>
		</div>
		<h1>
			The docs subtree could not resolve this page.
		</h1>
		<p>
			The request matched the docs section, so the router resolved the nearest directory-scoped
			<span class="inline-code">not-found.gsx</span>
			instead of falling back to the root 404 page.
		</p>
		<div class="hero-actions">
			<a href="/docs/introduction" class="cta-link primary" data-gosx-link="true">Open the introduction</a>
			<a href="/docs/getting-started" class="cta-link" data-gosx-link="true">Open getting started</a>
		</div>
	</article>
}
