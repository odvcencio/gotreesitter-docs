package authoring

func Page() Node {
	return <section class="page">
		<span class="eyebrow">Grammar Authoring</span>
		<h1 class="h1">Write a grammar. Watch it compile.</h1>
		<div class="underbar"></div>
		<p class="lead">
			Edit a tree-sitter grammar.json below. grammargen — gotreesitter's grammar compiler — compiles it into an LR table and a live Language entirely inside this WebAssembly build, then parses your sample against it. Nothing leaves this device.
		</p>
		<Surface
			name="GotreesitterAuthoring"
			runtime="go-wasm"
			wasmPath={data.wasmURL}
			capabilities="wasm text-input"
			requiredCapabilities="wasm"
			id="ag-root"
			class="ag-surface"
		>
			<div class="ag-toolbar">
				<label class="ag-anonlabel" title="Include anonymous nodes (literal tokens) in the rendered tree">
					<input id="ag-anonymous" type="checkbox" checked />
					anonymous nodes
				</label>
				<span class="tspacer"></span>
				<span id="ag-node-count" class="hlcredit">waiting for runtime</span>
			</div>
			<div class="playgrid ag-grid">
				<div class="panel">
					<div class="panelhd">
						<span class="cdot r"></span>
						<span class="cdot y"></span>
						<span class="cdot g"></span>
						grammar.json
					</div>
					<div class="panelbd ag-editorwrap">
						<textarea
							id="ag-grammar"
							class="ag-src mono"
							wrap="off"
							spellcheck="false"
							aria-label="Grammar JSON"
							maxlength="65536"
						>{data.grammar}</textarea>
					</div>
				</div>
				<div class="panel">
					<div class="panelhd">
						<span class="ldot c-cyan" style="border-color:var(--paper)"></span>
						sample source
					</div>
					<div class="panelbd ag-editorwrap ag-sample-wrap">
						<textarea
							id="ag-source"
							class="ag-src mono"
							wrap="off"
							spellcheck="false"
							aria-label="Sample source"
							maxlength="65536"
						>{data.sample}</textarea>
					</div>
				</div>
			</div>
			<div class="ag-results" aria-live="polite">
				<div class="panel">
					<div class="panelhd">
						<span class="ldot c-violet" style="border-color:var(--paper)"></span>
						syntax tree
					</div>
					<div class="panelbd ag-treewrap">
						<div id="ag-tree" class="tree ag-tree mono" role="tree" aria-label="Syntax tree">
							<p class="ag-tree-empty">Loading the browser grammar compiler…</p>
						</div>
					</div>
				</div>
				<div class="panel ag-diagpanel">
					<div class="panelhd">
						<span class="ldot c-red" style="border-color:var(--paper)"></span>
						diagnostics
					</div>
					<div class="panelbd ag-diagbd">
						<p id="ag-status" class="ag-status" role="status">Starting gotreesitter grammar compiler…</p>
						<div id="ag-errors"></div>
					</div>
				</div>
			</div>
		</Surface>
		<p class="p mut ag-footnote">
			Runtime: gotreesitter
			{data.gtsVersion}
			grammargen, compiled to standard Go WebAssembly and managed by GoSX. This is a Phase 0 spike: the compiler runs on the main thread, so a pathological grammar can stall the tab — keep grammars small for now. Grammar JSON and sample source never leave the browser.
		</p>
	</section>
}
