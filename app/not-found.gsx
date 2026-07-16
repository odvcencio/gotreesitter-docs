package docs

func Page() Node {
	return <article class="prose">
		<div class="page-topper">
			<span class="eyebrow">Missing</span>
			<p class="lede">
				The requested page does not exist in this docs tree.
			</p>
		</div>
		<h1>
			The page you asked for is not in this route tree.
		</h1>
		<p>
			The docs site is file-routed. If a page is missing, either the file does not exist or the URL needs a redirect rule.
		</p>
		<div class="hero-actions">
			<a href="/" class="cta-link primary" data-gosx-link="true">Return to overview</a>
			<a href="/docs/getting-started" class="cta-link" data-gosx-link="true">Open getting started</a>
		</div>
	</article>
}
