package docs

// Page renders the landing route ("/") — a direct, high-fidelity port of
// design/GoTreeSitter-Docs.html's `<sc-if value="{{ isHome }}">` section:
// hero (heromark/eyebrow/herotag/install/pillrow) + intro code block, the
// four tree-sitter-promises win cards, the benchmark numbers, the feature
// grid, and the 206-grammars teaser + foot. Renders inside the root
// layout's `.main` (see app/layout.gsx), so no topbar/sidebar here.
func Page() Node {
	return <section class="page">
		<div class="hero">
			<div>
				<span class="eyebrow">Incremental parsing library · pure Go</span>
				<h1 class="heromark"><span class="s1">go</span><span class="s2">tree</span><span class="s3">sitter</span></h1>
				<p class="herotag">
					Tree-sitter is a parser generator tool and an incremental parsing library.
					<b>gotreesitter is its runtime, reimplemented in pure Go</b> — the same
					parse-table format, the same <span class="mono">.scm</span> queries, ABI 15,
					no C toolchain.
				</p>
				<div class="install">
					<div class="cmd mono"><span class="pr">$</span> go get github.com/odvcencio/gotreesitter</div>
					<button class="copy" onclick="navigator.clipboard.writeText('go get github.com/odvcencio/gotreesitter').catch(()=>0);this.textContent='Copied';setTimeout(()=>this.textContent='Copy',1400)">Copy</button>
				</div>
				<div class="pillrow">
					<span class="pill"><b class="t-violet">206</b> grammars</span>
					<span class="pill"><b class="t-green">0</b> lines of C</span>
					<span class="pill"><b class="t-blue">1.54ms</b> full parse</span>
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
				<pre class="codebody"><span class="tk-kw">import</span> (
    <span class="tk-str">"fmt"</span>{"\n    "}<span class="tk-str">"github.com/odvcencio/gotreesitter"</span>{"\n    "}<span class="tk-str">"github.com/odvcencio/gotreesitter/grammars"</span>
)

lang   <span class="tk-op">:=</span> grammars.<span class="tk-fn">GoLanguage</span>()
parser <span class="tk-op">:=</span> gotreesitter.<span class="tk-fn">NewParser</span>(lang)

tree, _ <span class="tk-op">:=</span> parser.<span class="tk-fn">Parse</span>(src)
fmt.<span class="tk-fn">Println</span>(tree.<span class="tk-fn">RootNode</span>())
<span class="tk-cm">// (source_file (function_declaration ...))</span></pre>
			</div>
		</div>

		<h2 class="h2">Tree-sitter's four promises, <span class="t-pink">kept in Go.</span></h2>
		<div class="underbar"></div>
		<p class="p mut">
			tree-sitter set out to be general, fast, robust, and dependency-free. gotreesitter
			holds every one — as pure Go, against the same parse tables.
		</p>
		<div class="winrow">
			<div class="win">
				<div class="wtop c-cyan"></div>
				<h4>① General</h4>
				<p>
					Parse any language. gotreesitter loads the <b>same parse-table format</b> as the
					C runtime — 206 grammars in the registry, the same <span class="mono">.scm</span>
					queries, ABI 15 reserved-word sets.
				</p>
				<span class="pillref">tree-sitter → "general enough to parse any programming language"</span>
			</div>
			<div class="win">
				<div class="wtop c-green"></div>
				<h4>② Fast</h4>
				<p>
					Fast enough to parse on every keystroke. A single-byte incremental edit
					reparses in <b>649 ns</b> — <b>158×</b> faster than the C runtime — and a no-op
					reparse returns in nanoseconds with zero allocations. Full parse is
					competitive on representative files and slower on adversarial giants;
					the <a href="/docs/performance" data-gosx-link>numbers, with asterisks</a>.
				</p>
				<span class="pillref">tree-sitter → "fast enough to parse on every keystroke"</span>
			</div>
			<div class="win">
				<div class="wtop c-orange"></div>
				<h4>③ Robust</h4>
				<p>
					Useful results even with syntax errors. The generalized GLR core reproduces
					C's recovery <i>decision-for-decision</i>, verified against a v0.25.0 oracle for
					about 124 elected grammars.
				</p>
				<span class="pillref">tree-sitter → "robust enough to provide useful results with errors"</span>
			</div>
			<div class="win">
				<div class="wtop c-violet"></div>
				<h4>④ Dependency-free</h4>
				<p>
					tree-sitter's C runtime embeds in any application; the Go runtime
					<b>cross-compiles anywhere</b> — any GOOS/GOARCH incl. <span class="mono">wasip1</span>,
					no CGo, and fully visible to <span class="mono">-race</span>.
				</p>
				<span class="pillref">tree-sitter → "dependency-free … embedded in any application"</span>
			</div>
		</div>

		<h2 class="h2">The numbers</h2>
		<div class="underbar"></div>
		<p class="p mut">500-function Go source (19,294 bytes). Medians of 10 runs, Intel Core Ultra 9 285, GOMAXPROCS=1.</p>
		<p class="p mut">
			This is one representative corpus, not a universal claim. On deliberately adversarial
			files — the largest 8 files per language from real repositories — full parse runs from
			1.5× to 39× <i>slower</i> than C depending on the grammar. Incremental parsing is the
			category win and holds everywhere. The full spread is on the
			<a href="/docs/performance" data-gosx-link>performance page</a>, including the cases
			we still lose.
		</p>
		<div class="bench">
			<div class="brow">
				<div class="blabel"><span>FULL PARSE — lower is better</span><span class="mut">ms</span></div>
				<div class="btrack">
					<div class="bbar">
						<span class="bname">CGo binding</span>
						<div class="bfillwrap"><div class="bfill c-red" style="width:100%">~2.00</div></div>
					</div>
					<div class="bbar">
						<span class="bname">Native C</span>
						<div class="bfillwrap"><div class="bfill c-orange" style="width:88%">1.76</div></div>
					</div>
					<div class="bbar">
						<span class="bname t-green">gotreesitter</span>
						<div class="bfillwrap"><div class="bfill c-green" style="width:77%">1.54</div></div>
					</div>
				</div>
			</div>
		</div>
		<div class="multrow">
			<div class="mult c-pink" style="background:#ffe0ec">
				<div class="big t-pink">158×</div>
				<div class="sub">faster incremental single-byte edit vs native C — <b>649 ns</b>, 3 allocs</div>
			</div>
			<div class="mult" style="background:#d6f7ea">
				<div class="big t-green">41,800×</div>
				<div class="sub">faster no-edit reparse — <b>2.43 ns</b>, 0 B/op, 0 allocs</div>
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
			All 206 pass the curated C-oracle structural matrix, with no known-degraded
			skips. 116 have hand-written Go external scanners; 7 use hand-written Go token sources.
			<a href="/docs/languages" data-gosx-link>Browse the full registry →</a>
		</p>
		<div class="langteaser">
			<Each as="l" of={langTeaser}>
				<span class="lchip"><span class="t-blue">▪</span> {l}</span>
			</Each>
		</div>

		<div class="foot">
			<span>gotreesitter · pure-Go tree-sitter runtime · MIT</span>
			<span>{gtsVersion}</span>
		</div>
	</section>
}
