package playground

func Page() Node {
	return <section class="page">
		<span class="eyebrow">Playground</span>
		<h1 class="h1">The parser, in your browser.</h1>
		<div class="underbar"></div>
		<p class="lead">
			This is the production gotreesitter parser and query engine compiled from Go to WebAssembly. GoSX owns its lifecycle; parsing stays entirely on this device. Source and query text are never submitted to the server unless you copy a share link, which carries them in its URL.
		</p>
		<Surface
			name="GotreesitterPlayground"
			runtime="go-wasm"
			wasmPath={data.wasmURL}
			grammarIndexURL={data.grammarIndexURL}
			capabilities="wasm text-input fetch clipboard"
			requiredCapabilities="wasm fetch"
			id="pg-root"
			data-sample={data.sample}
			class="pg-surface"
		>
			<div class="pg-toolbar">
				<label class="pg-picker">
					<span class="pg-picklabel">language</span>
					<select id="pg-language" class="pg-select" aria-label="Language" disabled>
						<option value={data.language} selected>loading grammar index…</option>
					</select>
				</label>
				<label class="pg-anonlabel pg-anon-light" title="Include anonymous nodes in the rendered tree">
					<input id="pg-anonymous" type="checkbox" />
					anonymous nodes
				</label>
				<span class="tspacer"></span>
				<button id="pg-share" type="button" class="cta-link pg-share" disabled>Copy share link</button>
				<span id="pg-share-status" class="pg-share-status mono" role="status" aria-live="polite"></span>
				<button id="pg-parse" type="button" class="cta-link primary pg-submit">Parse now</button>
			</div>
			<If when={data.shareError != ""}>
				<p class="pg-share-note" role="status">{data.shareError}</p>
			</If>
			<div class="playgrid pg-grid pg-input-grid">
				<div class="panel">
					<div class="panelhd">
						<span class="cdot r"></span>
						<span class="cdot y"></span>
						<span class="cdot g"></span>
						source
						<span id="pg-language-label" class="hlcredit">{data.language}</span>
					</div>
					<div class="panelbd pg-editorwrap">
						<pre id="pg-hl" class="pg-hlpre mono" aria-hidden="true">{data.source}</pre>
						<textarea
							id="pg-source"
							data-shared={data.shared}
							class="pg-src mono"
							wrap="off"
							spellcheck="false"
							aria-label="Source code"
							maxlength="65536"
						>{data.source}</textarea>
					</div>
				</div>
				<div class="panel pg-qpanel">
					<div class="panelhd">
						<span class="ldot c-cyan" style="border-color:var(--paper)"></span>
						query
						<span class="hlcredit">Query.Execute · predicates supported</span>
					</div>
					<div class="panelbd pg-qbd">
						<textarea
							id="pg-query"
							class="pg-qsrc mono"
							wrap="off"
							spellcheck="false"
							rows="5"
							aria-label="Tree-sitter query"
						>{data.query}</textarea>
					</div>
				</div>
			</div>
			<div class="pg-results" aria-live="polite">
				<div class="panel">
					<div class="panelhd">
						<span class="ldot c-violet" style="border-color:var(--paper)"></span>
						syntax tree
						<span id="pg-node-count" class="hlcredit">waiting for runtime</span>
					</div>
					<div class="panelbd pg-treewrap">
						<div id="pg-tree" class="tree pg-tree mono" role="tree" aria-label="Syntax tree">
							<p class="pg-tree-empty">Loading the browser parser…</p>
						</div>
					</div>
				</div>
				<div class="panel pg-qresults">
					<div class="panelhd">
						<span class="ldot c-cyan" style="border-color:var(--paper)"></span>
						query results
						<span id="pg-capture-count" class="hlcredit">0 captures</span>
					</div>
					<div class="panelbd pg-qside pg-qresult-body">
						<p id="pg-status" class="pg-status" role="status">Starting gotreesitter WebAssembly…</p>
						<div id="pg-errors"></div>
						<div id="pg-captures" class="pg-captures"></div>
					</div>
				</div>
			</div>
		</Surface>
		<p class="p mut pg-footnote">
			Runtime: gotreesitter
			{data.gtsVersion}
			, compiled to standard Go WebAssembly and managed by GoSX. Network requests carry runtime and on-demand grammar assets only—never editor contents. A share link is the one exception: it encodes your source in the URL you choose to copy.
		</p>
	</section>
}
