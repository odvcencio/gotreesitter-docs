package docs

func Page() Node {
	return <section class="page home-page">
		<section class="home-hero" aria-labelledby="home-title">
			<div class="home-intro">
				<p class="eyebrow">gotreesitter · Go · v0.55.1</p>
				<h1 class="home-title" id="home-title">Read the code. Follow the tree.</h1>
				<p class="home-lede">
					gotreesitter parses Tree-sitter grammars in Go. Choose a language, point at a token, and follow it to the named node and its byte range.
				</p>
				<p class="home-facts">
					Released 2026-09-26 · 206 grammars · 119 Go external scanners · 7 token sources · no CGo in the runtime · MIT
				</p>
			</div>
			<div class="home-parse-heading">
				<span class="section-kicker">A real parse from the pinned runtime</span>
				<h2>Six samples. Source beside its syntax tree.</h2>
			</div>
			{parserDemo()}
			<div class="home-install">
				<span class="install-label">Add the current release</span>
				<div class="install" aria-label="Install gotreesitter v0.55.1">
					<code class="cmd mono"><span class="pr" aria-hidden="true">$ </span>go get github.com/odvcencio/gotreesitter@v0.55.1</code>
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
				<span class="section-kicker">Measurements with their scope</span>
				<h2 class="h2" id="receipts-title">Two dated receipts</h2>
			</div>
			<div class="receipt-grid">
				<article class="receipt-card">
					<div class="receipt-card-head">
						<h3>Quiet-host Go control</h3>
						<time datetime="2026-09-27">2026-09-27</time>
					</div>
					<p>20 seeds on gts-bench-1, a Xeon Platinum 8481C, Go 1.26.4, GOMAXPROCS=1. The generated 500-function Go file never forks.</p>
					<dl class="receipt-measures">
						<div><dt>Full parse</dt><dd>8,686,195 ns <span>· 8 allocs</span></dd></div>
						<div><dt>Single-byte edit</dt><dd>177,281 ns <span>· 5 allocs</span></dd></div>
						<div><dt>No-edit reparse</dt><dd>8.318 ns <span>· 0 allocs</span></dd></div>
					</dl>
					<a href="/docs/performance" data-gosx-link="true">Control method and limits <span aria-hidden="true">→</span></a>
				</article>
				<article class="receipt-card receipt-sealed">
					<div class="receipt-card-head">
						<h3>Sealed Go and C comparison</h3>
						<time datetime="2026-08-02">2026-08-02</time>
					</div>
					<p>Hardware-attested on an AMD SEV Confidential Space host. Four frozen Go files; each lane parses and checks the root, without walking the full tree.</p>
					<dl class="receipt-measures">
						<div><dt>Production Go / C</dt><dd>4.815×</dd></div>
						<div><dt>Compact Go / C</dt><dd>3.986×</dd></div>
				</dl>
					<a href="/docs/performance" data-gosx-link="true">Receipt and caveats <span aria-hidden="true">→</span></a>
				</article>
			</div>
		</section>

		<section class="road-strip" aria-labelledby="road-title">
			<div>
				<span class="section-kicker">Road to v1</span>
				<h2 id="road-title">M0 closed · M1–M4 open</h2>
			</div>
			<p>The global default flips only at v1.0.0-rc.1. <a href="/docs/v1" data-gosx-link="true">Read the v1 roadmap <span aria-hidden="true">→</span></a></p>
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
			<span>gotreesitter · v0.55.1 · MIT</span>
			<a href="/docs/performance" data-gosx-link="true">Performance evidence <span aria-hidden="true">→</span></a>
			<a href="/docs/contributing#contributors" data-gosx-link="true">Contributors <span aria-hidden="true">→</span></a>
		</footer>
	</section>
}
