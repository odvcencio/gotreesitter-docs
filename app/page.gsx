package docs

// Page renders the landing route ("/") using the current source contract:
// hero (heromark/eyebrow/herotag/install/pillrow) + intro code block, the
// four tree-sitter-promises win cards, the benchmark numbers, the feature
// grid, and the 206-grammars teaser + foot. Renders inside the root
// layout's `.main` (see app/layout.gsx), so no topbar/sidebar here.
func Page() Node {
	return <section class="page">
		<div class="hero">
			<div>
				<span class="eyebrow">Incremental parsing library · pure Go</span>
				<h1 class="heromark">
					<span class="s1">go</span>
					<span class="s2">tree</span>
					<span class="s3">sitter</span>
				</h1>
				<p class="herotag">
					Tree-sitter is a parser generator tool and an incremental parsing library.
					<b>
						gotreesitter is its runtime, reimplemented in pure Go
					</b>
					— the same parse-table format, the same
					<span class="mono">.scm</span>
					queries, ABI 15, no C toolchain.
				</p>
				<div class="install">
					<div class="cmd mono">
						<span class="pr">$</span>
						go get github.com/odvcencio/gotreesitter
					</div>
				</div>
				<div class="pillrow">
					<span class="pill">
						<b class="t-violet">206</b>
						grammars
					</span>
					<span class="pill">
						<b class="t-green">0</b>
						lines of C
					</span>
					<span class="pill">
						<b class="t-blue">0</b>
						edit allocations
					</span>
					<span class="pill">MIT</span>
				</div>
			</div>
			<div class="code">
				<div class="codehead">
					<span class="cdot r"></span>
					<span class="cdot y"></span>
					<span class="cdot g"></span>
					<span class="cfile mono">main.go</span>
					<span class="clang">go</span>
				</div>
				<pre class="codebody">{heroCode()}</pre>
			</div>
		</div>
		<h2 class="h2">
			Tree-sitter's four promises,
			<span class="t-pink">kept in Go.</span>
		</h2>
		<div class="underbar"></div>
		<p class="p mut">
			tree-sitter set out to be general, fast, robust, and dependency-free. gotreesitter holds every one — as pure Go, against the same parse tables.
		</p>
		<div class="winrow">
			<div class="win">
				<div class="wtop c-cyan"></div>
				<h4>① General</h4>
				<p>
					Parse any language. gotreesitter loads the
					<b>same parse-table format</b>
					as the C runtime — 206 grammars in the registry, the same
					<span class="mono">.scm</span>
					queries, ABI 15 reserved-word sets.
				</p>
				<span class="pillref">
					tree-sitter → "general enough to parse any programming language"
				</span>
			</div>
			<div class="win">
				<div class="wtop c-green"></div>
				<h4>② Fast</h4>
				<p>
					Fast enough to parse on every keystroke. On the pinned-host receipt, a single-byte edit takes
					<b>1.98 µs</b>
					—
					<b>5,500×</b>
					faster than a full parse of the same file — and a no-op reparse takes 9.9 ns. Both allocate zero. Full parse is slower than C on the canonical workload and across much of the fleet; the
					<a href="/docs/performance" data-gosx-link="true">numbers, with asterisks</a>
					.
				</p>
				<span class="pillref">
					tree-sitter → "fast enough to parse on every keystroke"
				</span>
			</div>
			<div class="win">
				<div class="wtop c-orange"></div>
				<h4>③ Robust</h4>
				<p>
					Useful results even with syntax errors. The generalized GLR core and language-specific scanners are checked against a pinned C oracle. All 206 grammars pass the curated structural matrix; real-corpus gaps stay explicit.
				</p>
				<span class="pillref">
					tree-sitter → "robust enough to provide useful results with errors"
				</span>
			</div>
			<div class="win">
				<div class="wtop c-violet"></div>
				<h4>④ Dependency-free</h4>
				<p>
					tree-sitter's C runtime embeds in any application; the Go runtime
					<b>cross-compiles anywhere</b>
					— any GOOS/GOARCH incl.
					<span class="mono">wasip1</span>
					, no CGo, and fully visible to
					<span class="mono">-race</span>
					.
				</p>
				<span class="pillref">
					tree-sitter → "dependency-free … embedded in any application"
				</span>
			</div>
		</div>
		<h2 class="h2">The numbers</h2>
		<div class="underbar"></div>
		<p class="p mut">
			Full-parse headline: four frozen, human-written Go files (5 KB–236 KB), each measured against a locked upstream tree-sitter v0.25.1 C oracle — ten process-isolated samples per fixture, byte-exact tree identity admitted before every timing run.
		</p>
		<p class="p mut">
			Full parse is currently slower than the C runtime on that real-code matrix — that's the honest number, not a marketing one; gotreesitter wins on the editor-style incremental path below instead. An older generated 500-function file is retained only for tracking single-stack regressions, not as a full-parse headline. Fleet ratios vary widely by grammar; the complete distribution and every named gap are on the
			<a href="/docs/performance" data-gosx-link="true">performance page</a>
			.
		</p>
		<div class="mult" style="background:#ffedd0">
			<div class="big t-orange">4.85×</div>
			<div class="sub">
				slower than
				<b>C</b>
				— full parse, real-code equal-fixture geomean across the four frozen fixtures. 204/206 grammars ratcheted, 206/206 pass curated structural parity.
			</div>
		</div>
		<div class="multrow">
			<div class="mult c-pink" style="background:#ffe0ec">
				<div class="big t-pink">5,500×</div>
				<div class="sub">
					faster single-byte edit vs a full parse of the same file —
					<b>1.98 µs</b>
					, 0 allocs
				</div>
			</div>
			<div class="mult" style="background:#d6f7ea">
				<div class="big t-green">~1.1M×</div>
				<div class="sub">
					faster no-edit reparse vs a full parse of the same file —
					<b>9.9 ns</b>
					, 0 B/op, 0 allocs
				</div>
			</div>
		</div>
		<h2 class="h2">A whole parsing toolkit</h2>
		<div class="underbar"></div>
		<div class="grid3">
			<Each as="f" of={features}>
				<div class="feat">
					<div class={"featicon " + f.color}>{f.tok}</div>
					<h4>{f.ttl}</h4>
					<p>{f.body}</p>
				</div>
			</Each>
		</div>
		<h2 class="h2">206 grammars, embedded</h2>
		<div class="underbar"></div>
		<p class="p">
			All 206 pass the curated C-oracle structural matrix, with no known-degraded skips. 116 have hand-written Go external scanners; 7 use hand-written Go token sources.
			<a href="/docs/languages" data-gosx-link="true">Browse the full registry →</a>
		</p>
		<div class="langteaser">
			<Each as="l" of={langTeaser}>
				<span class="lchip">
					<span class="t-blue">▪</span>
					{l}
				</span>
			</Each>
		</div>
		<div class="foot">
			<span>
				gotreesitter · pure-Go tree-sitter runtime · MIT
			</span>
			<span>{gtsVersion}</span>
		</div>
	</section>
}
