package docs

func Page() Node {
	return <article class="prose">
		<div class="page-topper">
			<span class="eyebrow">GoTreeSitter</span>
			<p class="lede">
				A pure-Go, byte-exact reimplementation of tree-sitter — no CGo, no C toolchain, 206 grammars built in.
			</p>
		</div>
		<h1>
			GoTreeSitter documentation
		</h1>
		<p>
			Parsing, incremental re-parse, queries, language grammars, and the internals that keep
			output byte-exact with C tree-sitter. Start with the introduction, or jump straight to
			getting a file parsed.
		</p>
		<div class="hero-actions">
			<a href="/docs/introduction" data-gosx-link class="cta-link primary">Read the introduction</a>
			<a href="/docs/getting-started" data-gosx-link class="cta-link">Get started</a>
		</div>
	</article>
}
