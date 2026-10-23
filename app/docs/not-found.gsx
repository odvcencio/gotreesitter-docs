package docs

func Page() Node {
	return <section class="page notfound">
		<span class="eyebrow">Docs · 404</span>
		<h1 class="h1">This docs page does not exist.</h1>
		<p class="lead">The /docs tree is file-routed; a missing page means the file does not exist or needs a redirect.</p>
		<form class="nf-search" method="get" action="/changelog" role="search" aria-label="Search the release history" data-gosx-form-mode="get">
			<label class="nf-label" for="nf-q">Search releases and changes</label>
			<div class="nf-row">
				<input id="nf-q" class="change-input" type="search" name="q" placeholder="incremental, YAML, #1435 …" />
				<button class="change-submit" type="submit">Search</button>
			</div>
		</form>
		<h2 class="h2">Try the parser instead</h2>
		<nav class="playground-language-links" aria-label="Open a playground sample">
			<a href="/playground?lang=go&sample=add" data-gosx-link="true">Go <span aria-hidden="true">↗</span></a>
			<a href="/playground?lang=python&sample=add" data-gosx-link="true">Python <span aria-hidden="true">↗</span></a>
			<a href="/playground?lang=rust&sample=add" data-gosx-link="true">Rust <span aria-hidden="true">↗</span></a>
			<a href="/playground?lang=typescript&sample=add" data-gosx-link="true">TypeScript <span aria-hidden="true">↗</span></a>
			<a href="/playground?lang=json&sample=add" data-gosx-link="true">JSON <span aria-hidden="true">↗</span></a>
			<a href="/playground?lang=html&sample=add" data-gosx-link="true">HTML <span aria-hidden="true">↗</span></a>
		</nav>
		<h2 class="h2">Start from the guides</h2>
		<div class="hero-actions">
			<a href="/docs/introduction" class="cta-link primary" data-gosx-link="true">Introduction</a>
			<a href="/docs/getting-started" class="cta-link" data-gosx-link="true">Getting started</a>
			<a href="/docs/languages" class="cta-link" data-gosx-link="true">Search the 206 languages</a>
		</div>
	</section>
}
