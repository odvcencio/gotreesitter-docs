package playground

// The live playground: a canonical GoSX surface backed by a dedicated
// standard-Go WASM engine. The server-rendered children are the complete
// fallback; app/playground/client enhances them with the production parser,
// persistent incremental document, query/highlight UI, and bounded detection.
func Page() Node {
	return <section class="page">
		<span class="eyebrow">Playground</span>
		<h1 class="h1">The parser, in your tab.</h1>
		<div class="underbar"></div>
		<p class="lead">
			The actual production parser, compiled to WASM, running in your tab. Pick a language or just start typing — it'll figure it out, and queries run live, predicates included. Parsing and query execution stay local; auto-detection may send a bounded source sample to the server.
		</p>
		<If when={data.wasmReady == false}>
			<div class="note new">
				<span class="tag">Runtime not staged</span>
				<p class="p" style="margin:0">
					<b class="mono">public/playground/runtime.wasm</b>
					is missing, so the editor below cannot boot. Run
					<b class="mono">./scripts/build-playground-wasm.sh</b>
					and reload.
				</p>
			</div>
		</If>
		<Surface
			name="GoTreesitterPlayground"
			runtime="go-wasm"
			wasmPath={data.wasmURL}
			capabilities="wasm fetch keyboard text-input"
			requiredCapabilities="wasm fetch"
			version={data.gtsVersion}
			languagesURL={data.languagesURL}
			languageURLPrefix={data.languageURLPrefix}
			detectURL={data.detectURL}
		>
			<div id="pg-root" class="pg-root">
				<div class="pg-toolbar">
					<label class="pg-picker">
						<span class="pg-picklabel">language</span>
						<select id="pg-lang" class="pg-select" aria-label="Language (Auto-detect by default)" disabled>
							<option value="">Auto-detect</option>
						</select>
					</label>
					<span id="pg-badge" class="pg-badge" hidden></span>
					<span class="tspacer"></span>
					<span id="pg-status" class="pg-status" role="status" aria-live="polite" aria-atomic="true">
						<i id="pg-dot" class="pg-dot"></i>
						<span id="pg-stat">booting runtime…</span>
					</span>
				</div>
				<div class="pg-tryrow">
					<span class="pg-trylabel">try:</span>
					<button type="button" class="qchip" data-sample="go">go</button>
					<button type="button" class="qchip" data-sample="python">python</button>
					<button type="button" class="qchip" data-sample="json">json</button>
					<button type="button" class="qchip" data-sample="markdown">markdown</button>
				</div>
				<div class="playgrid pg-grid">
					<div class="panel">
						<div class="panelhd">
							<span class="cdot r"></span>
							<span class="cdot y"></span>
							<span class="cdot g"></span>
							source
							<span id="pg-langtag" class="hlcredit">auto</span>
						</div>
						<div class="panelbd pg-editorwrap">
							<pre id="pg-hl" class="pg-hlpre mono" aria-hidden="true"></pre>
							<textarea
								id="pg-src"
								class="pg-src mono"
								wrap="off"
								spellcheck="false"
								autocomplete="off"
								autocapitalize="off"
								autocorrect="off"
								aria-label="Source code"
								disabled
							>{data.initialSource}</textarea>
							<div id="pg-loading" class="pg-loading mono">booting…</div>
						</div>
					</div>
					<div class="panel">
						<div class="panelhd">
							<span class="ldot c-violet" style="border-color:var(--paper)"></span>
							syntax tree
							<label class="pg-anonlabel" title="Include anonymous (unnamed) nodes in the tree">
								<input type="checkbox" id="pg-anon" />
								anonymous
							</label>
							<span class="hlcredit">tree.RootNode()</span>
						</div>
						<div class="panelbd pg-treewrap">
							<div id="pg-tree" class="tree pg-tree mono" role="tree" aria-label="Syntax tree"></div>
						</div>
					</div>
				</div>
				<div class="panel pg-qpanel">
					<div class="panelhd">
						<span class="ldot c-cyan" style="border-color:var(--paper)"></span>
						query
						<button
							type="button"
							id="pg-qcollapse"
							class="pg-qcollapse"
							aria-expanded="true"
							aria-controls="pg-qbd"
						>hide</button>
						<span class="hlcredit">Query.Execute · predicates run live</span>
					</div>
					<div class="panelbd pg-qbd" id="pg-qbd">
						<div class="pg-qgrid">
							<textarea
								id="pg-query"
								class="pg-qsrc mono"
								wrap="off"
								spellcheck="false"
								autocomplete="off"
								autocapitalize="off"
								autocorrect="off"
								rows="5"
								aria-label="Tree-sitter query"
								placeholder="(function_declaration name: (identifier) @fn)"
							></textarea>
							<div class="pg-qside">
								<div id="pg-qerr" class="pg-qerr mono" role="alert" hidden></div>
								<div id="pg-legend" class="pg-legend mono" hidden></div>
								<button type="button" id="pg-qhint" class="pg-qhint mono">
									try a predicate: ((identifier) @id (#match? @id "^ma"))
								</button>
							</div>
						</div>
					</div>
				</div>
				<p class="p mut pg-footnote">
					Runtime: gotreesitter
					{data.gtsVersion}
					compiled to js/wasm (
					{data.wasmMB}
					download, cached after the first visit); grammars stream in per language as compiled blobs. Parsing and queries happen entirely in this tab. Auto-detection sends a bounded source sample to the server; select a language first to keep source fully local.
				</p>
			</div>
		</Surface>
	</section>
}
