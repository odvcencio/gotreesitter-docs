package docs

func Page() Node {
	return <section class="page home-page">
		<section class="home-hero" aria-labelledby="home-title">
			<div class="home-intro">
				<p class="eyebrow">gotreesitter · Go · v0.58.0</p>
				<h1 class="home-title" id="home-title">Read the code. Follow the tree.</h1>
				<p class="home-lede">
					gotreesitter parses Tree-sitter grammars in Go. Choose a language, point at a token, and follow it to the named node and its byte range.
				</p>
				<p class="home-facts">
					Released 2026-11-20 · 206 grammars · 119 Go external scanners · 4 default token sources · no CGo in the runtime · MIT
				</p>
			</div>
			<div class="home-parse-heading">
				<span class="section-kicker">A real parse from the pinned runtime</span>
				<h2>Six samples. Source beside its syntax tree.</h2>
			</div>
			{parserDemo()}
			<div class="home-install">
				<span class="install-label">Add the current release</span>
				<div class="install" aria-label="Install gotreesitter v0.58.0">
					<code class="cmd mono" tabindex="0" aria-label="Installation command; scroll horizontally to read the full command"><span class="pr" aria-hidden="true">$ </span>go get github.com/odvcencio/gotreesitter@v0.58.0</code>
				</div>
			</div>
		</section>

		<section class="playground-band" aria-labelledby="playground-band-title">
			<div class="playground-band-copy">
				<span class="section-kicker">Try a parse in your browser</span>
				<h2 id="playground-band-title">206 languages, parsed in your browser; your code never leaves the tab.</h2>
				<p>Open the GoSX playground with a starter sample. Parsing runs in WebAssembly on your device.</p>
			</div>
			<nav class="playground-language-links" aria-label="Open a playground sample">
				<a href="/playground?lang=go" data-gosx-link="true">Go <span aria-hidden="true">↗</span></a>
				<a href="/playground?lang=python" data-gosx-link="true">Python <span aria-hidden="true">↗</span></a>
				<a href="/playground?lang=rust" data-gosx-link="true">Rust <span aria-hidden="true">↗</span></a>
				<a href="/playground?lang=typescript" data-gosx-link="true">TypeScript <span aria-hidden="true">↗</span></a>
				<a href="/playground?lang=json" data-gosx-link="true">JSON <span aria-hidden="true">↗</span></a>
				<a href="/playground?lang=html" data-gosx-link="true">HTML <span aria-hidden="true">↗</span></a>
			</nav>
		</section>

		<section class="home-receipts" aria-labelledby="receipts-title">
			<div class="home-section-heading">
				<h2 class="h2" id="receipts-title">Certified parsing in v0.58.0</h2>
				<p>34 grammars use the certified core by default. The playground uses the published v0.58.0 module.</p>
			</div>
			<div class="receipt-grid">
				<article class="receipt-card">
					<div class="receipt-card-head"><h3>Fresh parses against C</h3></div>
					<p>Paired runs on the same locked files, about 2 KiB each. Values are Go time divided by pinned C time.</p>
					<dl class="receipt-measures">
						<div><dt>JSON</dt><dd>0.43×</dd></div>
						<div><dt>Python</dt><dd>0.44×</dd></div>
						<div><dt>Kotlin</dt><dd>0.38×</dd></div>
					</dl>
					<a href="/docs/performance#certified-core-comparisons" data-gosx-link="true">Inputs and benchmarks <span aria-hidden="true">→</span></a>
				</article>
				<article class="receipt-card">
					<div class="receipt-card-head"><h3>Reported editor workloads</h3></div>
					<p>CSS and TOML used the same identifier edit in both implementations. Diff and Make were checked separately for edits at the end of a file.</p>
					<dl class="receipt-measures">
						<div><dt>CSS edit</dt><dd>0.79× C time</dd></div>
						<div><dt>TOML edit</dt><dd>0.69× C time</dd></div>
					</dl>
					<a href="/docs/incremental-parsing" data-gosx-link="true">Integrate incremental parsing <span aria-hidden="true">→</span></a>
				</article>
			</div>
		</section>

		<section class="road-strip" aria-labelledby="road-title">
			<div><h2 id="road-title">Which parser will I use?</h2></div>
			<p>The same Go API selects the certified engine for eligible grammar requests. <a href="/docs/parser-engines" data-gosx-link="true">Certified grammars and upgrade steps <span aria-hidden="true">→</span></a></p>
		</section>

		<section class="home-features" aria-labelledby="features-title">
			<div class="home-section-heading">
				<span class="section-kicker">The runtime around the parser</span>
				<h2 class="h2" id="features-title">Tools for working with syntax trees</h2>
			</div>
			<div class="grid3 home-feature-grid">
				<Each as="f" of={features}>
					<article class="feat">
						<div class={"featicon " + f.color} aria-hidden="true">{f.tok}</div>
						<h3>{f.ttl}</h3>
						<p>{f.body}</p>
					</article>
				</Each>
			</div>
		</section>

		<footer class="home-footer">
			<span>gotreesitter · v0.58.0 · MIT</span>
			<a href="/docs/performance" data-gosx-link="true">Performance evidence <span aria-hidden="true">→</span></a>
			<a href="/docs/contributing#contributors" data-gosx-link="true">Contributors <span aria-hidden="true">→</span></a>
		</footer>
	</section>
}
