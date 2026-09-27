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
					— the tree-sitter parse-table format, familiar
					<span class="mono">.scm</span>
					queries, ABI 15 reserved-word tables, no C toolchain.
				</p>
				<div class="install">
					<div class="cmd mono">
						<span class="pr">$</span>
						go get github.com/odvcencio/gotreesitter<span>@</span>v0.55.1
					</div>
				</div>
				<div class="pillrow">
					<span class="pill">
						<b class="t-violet">206</b>
						grammars
					</span>
					<span class="pill">
						<b class="t-green">0</b>
						CGo
					</span>
					<span class="pill">
						<b class="t-blue">Go</b>
						runtime
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
			Tree-sitter's design,
			<span class="t-pink">implemented in Go.</span>
		</h2>
		<div class="underbar"></div>
		<p class="p mut">
			gotreesitter uses tree-sitter parse tables with a Go parser, lexer, query engine, and grammar registry.
		</p>
		<div class="winrow">
			<div class="win">
				<div class="wtop c-cyan"></div>
				<h4>① General</h4>
				<p>
					The registry contains 206 grammars and uses the same parse-table format as the C runtime. The runtime supports ABI 15 reserved-word tables.
				</p>
				<span class="pillref">
					tree-sitter → "general enough to parse any programming language"
				</span>
			</div>
			<div class="win">
				<div class="wtop c-green"></div>
				<h4>② Fast</h4>
				<p>
					On the 2026-09-27 control, a single-byte edit took
					<b>177,281 ns</b>
					and no-edit reuse took 8.318 ns. The generated input never forks. See the
					<a href="/docs/performance" data-gosx-link="true">dated benchmark evidence</a>.

				</p>
				<span class="pillref">
					tree-sitter → "fast enough to parse on every keystroke"
				</span>
			</div>
			<div class="win">
				<div class="wtop c-orange"></div>
				<h4>③ Robust</h4>
				<p>
					Useful results even with syntax errors. The generalized GLR core and language-specific scanners are checked against a pinned C oracle. The curated structural gate covers 206 grammars. The dated parity boards record other known differences.
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
			Version v0.55.1 uses production GLR parsing by default. Set GTS_ADMISSION_CANDIDATE=1 to try compact parsing; the language allowlist is empty.
		</p>
		<p class="p mut">
			The quiet-host control measured the exact v0.55.1 source, 92db945f, on 2026-09-27 at gts-bench-1. Full parse took 8,686,195 ns/op with 8 allocations; a single-byte edit took 177,281 ns/op with 5 allocations; no-edit reuse took 8.318 ns/op with 0 allocations. Its generated 500-function Go file never forks, so it is a control, not typical code. See the performance page for the command and host details. The hardware-attested v9 receipt measured four frozen Go files on 2026-08-02 at build 492cd600 in an AMD SEV Confidential Space VM: 4.815x C for production and 3.986x C for compact. It is not a v0.55.1 measurement and cannot be rerun outside Confidential Space. See the
			<a href="/docs/performance" data-gosx-link="true">performance page</a> for methods and limits.
		</p>
		<div class="mult" style="background:#ffedd0">
			<div class="big t-orange">v0.55.1</div>
			<div class="sub">Production GLR is the default. Compact parser graduation remains in progress.</div>
		</div>
		<div class="multrow">
			<div class="mult c-pink" style="background:#ffe0ec">
				<div class="big t-pink">177,281 ns</div>
				<div class="sub">Single-byte edit: 5 allocations. Generated Go control, 2026-09-27, exact source 92db945f, gts-bench-1.</div>
			</div>
			<div class="mult" style="background:#d6f7ea">
				<div class="big t-green">8.318 ns</div>
				<div class="sub">No-edit reuse: 0 allocations. Same control receipt; generated input never forks.</div>
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
			The v0.55.1 language guide lists 206 registry grammars, 119 Go external scanners, and 7 token-source implementations. Its smoke samples do not establish parity for every input.
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
