// /playground client logic. Dependency-free; pairs with
// app/playground/page.gsx (skeleton) and app/playground_api.go (JSON data
// plane). wasm_exec.js must be loaded first (both are <script defer> in
// page order).
//
// Boot: fetch /playground/langs.json (fills the picker) while streaming
// runtime.wasm with a progress readout, instantiate, wait for the
// `gotreesitter` global, then enable the editor and parse the initial
// buffer. Languages are activated lazily: /playground/lang/{name}.json →
// base64 blob → gotreesitter.loadBlob(name, bytes, highlightQuery), cached
// client-side per language.
//
// The runtime's parse() returns a structured tree as a JSON string: nodes
// carry BOTH byte spans (start/end — offsets into the UTF-8 encoding, used
// to paint the overlay) and UTF-16 spans (start16/end16 — offsets into the
// JS string, used for textarea selection). query() compiles + executes a
// tree-sitter query with the engine's native predicate evaluation.
//
// Auto-detect (default until the visitor picks a language): instant
// signature heuristics on every keystroke, plus a 600ms-debounced
// POST /playground/detect parse-race for anything the signatures miss.
(() => {
	"use strict";

	const root = document.getElementById("pg-root");
	if (!root || root.dataset.pgBooted) return;
	root.dataset.pgBooted = "1";

	const ui = {
		lang: document.getElementById("pg-lang"),
		badge: document.getElementById("pg-badge"),
		dot: document.getElementById("pg-dot"),
		stat: document.getElementById("pg-stat"),
		langtag: document.getElementById("pg-langtag"),
		hl: document.getElementById("pg-hl"),
		src: document.getElementById("pg-src"),
		tree: document.getElementById("pg-tree"),
		loading: document.getElementById("pg-loading"),
		anon: document.getElementById("pg-anon"),
		query: document.getElementById("pg-query"),
		qerr: document.getElementById("pg-qerr"),
		legend: document.getElementById("pg-legend"),
		qhint: document.getElementById("pg-qhint"),
		qcollapse: document.getElementById("pg-qcollapse"),
		qbd: document.getElementById("pg-qbd"),
	};

	const state = {
		ready: false,
		auto: true, // auto-detect until the picker is used
		manualLang: null, // picker choice when auto is off
		detected: "go", // current auto-mode choice
		activeLang: null, // language of the last successful parse
		loaded: new Map(), // name -> {hasHighlight}
		langsIndex: new Map(), // name -> {blobBytes}
		version: root.dataset.version || "dev",
		parseSeq: 0,
		detectSeq: 0,
		// Structured-tree / query state. renderedSource is the buffer of the
		// last successful parse — the tree, token ranges and captures are all
		// coordinates into it; it goes null the moment the buffer changes.
		tree: null,
		renderedSource: null,
		renderedLang: null,
		tokenRanges: null,
		captures: null, // {ranges, byName, nodeMarks, truncated}
		capturesFor: null, // renderedSource snapshot the captures were computed against
		flash: null, // {start,end} byte span pulsing in the overlay
		showAnon: false,
		queryOpen: true,
		visibleRows: [], // node objects, index-aligned with rendered rows
		activeRowEl: null,
		lastElapsed: null,
		lastDot: "ok",
	};

	// The go sample is the server-rendered initial buffer (single source of
	// truth in app/playground/page.server.go); the rest live here.
	const samples = {
		go: ui.src.value,
		python:
			'def fib(n):\n    a, b = 0, 1\n    for _ in range(n):\n        a, b = b, a + b\n    return a\n\n\nprint([fib(i) for i in range(10)])\n',
		json:
			'{\n  "name": "gotreesitter",\n  "grammars": 206,\n  "cgo": false,\n  "targets": ["js/wasm", "linux/arm64"]\n}\n',
		markdown:
			'# gotreesitter\n\nA **pure Go** reimplementation of tree-sitter.\n\n- 206 grammars in the box\n- zero lines of C\n\n```go\ntree, _ := parser.Parse(src)\n```\n',
	};

	const mb = (n) => (n / (1024 * 1024)).toFixed(1) + " MB";
	const kb = (n) => (n >= 1024 * 1024 ? mb(n) : Math.max(1, Math.round(n / 1024)) + " KB");

	function setLoading(text) {
		ui.loading.hidden = false;
		ui.loading.classList.remove("err");
		ui.loading.textContent = text;
	}

	function failHard(message) {
		ui.loading.hidden = false;
		ui.loading.classList.add("err");
		ui.loading.textContent = message;
		ui.stat.textContent = "runtime unavailable";
		ui.dot.className = "pg-dot err";
	}

	function setStat(text, dotClass) {
		ui.stat.textContent = text;
		ui.dot.className = "pg-dot" + (dotClass ? " " + dotClass : "");
	}

	function setBadge(text) {
		if (!text) {
			ui.badge.hidden = true;
			return;
		}
		ui.badge.hidden = false;
		ui.badge.textContent = text;
	}

	// ---- boot ---------------------------------------------------------

	async function instantiateWasm() {
		if (typeof WebAssembly === "undefined") {
			throw new Error("This browser has no WebAssembly support — the playground cannot run here.");
		}
		if (typeof Go === "undefined") {
			throw new Error("wasm_exec.js failed to load — the playground cannot run here.");
		}
		const url = root.dataset.wasmUrl;
		const expected = parseInt(root.dataset.wasmBytes || "0", 10);
		const resp = await fetch(url);
		if (!resp.ok) {
			throw new Error(
				"runtime.wasm is missing (HTTP " + resp.status + ").\nRun ./scripts/build-playground-wasm.sh and reload."
			);
		}
		let bytes;
		if (resp.body && resp.body.getReader) {
			const total = expected || parseInt(resp.headers.get("Content-Length") || "0", 10);
			const reader = resp.body.getReader();
			const chunks = [];
			let got = 0;
			for (;;) {
				const { done, value } = await reader.read();
				if (done) break;
				chunks.push(value);
				got += value.length;
				setLoading("downloading runtime…\n" + mb(got) + (total ? " / " + mb(total) : ""));
			}
			bytes = new Uint8Array(got);
			let off = 0;
			for (const chunk of chunks) {
				bytes.set(chunk, off);
				off += chunk.length;
			}
		} else {
			setLoading("downloading runtime…");
			bytes = new Uint8Array(await resp.arrayBuffer());
		}
		setLoading("instantiating runtime…");
		const go = new Go();
		const result = await WebAssembly.instantiate(bytes, go.importObject);
		go.run(result.instance); // resolves only when the runtime exits — do not await
		// The Go side installs the `gotreesitter` global from its main();
		// give it a moment to appear.
		const deadline = Date.now() + 10000;
		while (!(window.gotreesitter && typeof window.gotreesitter.parse === "function")) {
			if (Date.now() > deadline) throw new Error("runtime started but the gotreesitter global never appeared");
			await new Promise((r) => setTimeout(r, 25));
		}
	}

	function buildPicker(payload) {
		if (payload && payload.version) state.version = payload.version;
		const langs = (payload && payload.languages) || [];
		const frag = document.createDocumentFragment();
		for (const lang of langs) {
			state.langsIndex.set(lang.name, lang);
			const opt = document.createElement("option");
			opt.value = lang.name;
			opt.textContent = lang.name + (lang.blobBytes > 0 ? " · " + kb(lang.blobBytes) : " · unavailable");
			if (!lang.blobBytes) opt.disabled = true;
			frag.appendChild(opt);
		}
		ui.lang.appendChild(frag);
	}

	async function boot() {
		setLoading("fetching language index…");
		const langsPromise = fetch("/playground/langs.json").then((r) => {
			if (!r.ok) throw new Error("langs.json failed: HTTP " + r.status);
			return r.json();
		});
		await instantiateWasm();
		buildPicker(await langsPromise);
		state.ready = true;
		ui.src.disabled = false;
		ui.lang.disabled = false;
		ui.loading.hidden = true;
		setStat("runtime ready", "ok");
		runParse();
		scheduleDetect(); // populate the badge for the initial buffer
	}

	// ---- language activation ------------------------------------------

	async function ensureLanguage(name) {
		if (state.loaded.has(name)) return;
		const known = state.langsIndex.get(name);
		if (known && !known.blobBytes) throw new Error(name + " has no compiled blob");
		setStat("loading " + name + "…", "");
		const resp = await fetch(
			"/playground/lang/" + encodeURIComponent(name) + ".json?v=" + encodeURIComponent(state.version)
		);
		if (!resp.ok) throw new Error(name + ": blob fetch failed (HTTP " + resp.status + ")");
		const data = await resp.json();
		const bin = atob(data.blob);
		const bytes = new Uint8Array(bin.length);
		for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
		let hasHighlight = !!data.highlightQuery;
		let res = window.gotreesitter.loadBlob(data.name, bytes, data.highlightQuery || "");
		if (!res || !res.ok) {
			// A bad highlight query must not cost us the language: the
			// runtime registers the grammar before compiling the
			// highlighter, so retry with highlighting off.
			res = window.gotreesitter.loadBlob(data.name, bytes, "");
			hasHighlight = false;
		}
		if (!res || !res.ok) throw new Error((res && res.error) || name + ": loadBlob failed");
		state.loaded.set(data.name, { hasHighlight });
	}

	// ---- overlay rendering ----------------------------------------------

	function renderPlain(source) {
		ui.hl.textContent = source + "\n";
		syncScroll();
	}

	const tokClassTable = {
		keyword: "tk-kw",
		function: "tk-fn",
		method: "tk-fn",
		constructor: "tk-fn",
		string: "tk-str",
		char: "tk-str",
		escape: "tk-str",
		number: "tk-num",
		float: "tk-num",
		integer: "tk-num",
		boolean: "tk-kw",
		constant: "tk-kw",
		type: "tk-ty",
		tag: "tk-ty",
		attribute: "tk-ty",
		comment: "tk-cm",
		operator: "tk-op",
		punctuation: "tk-pn",
		delimiter: "tk-pn",
		variable: "tk-id",
		property: "tk-id",
		parameter: "tk-id",
		field: "tk-id",
		label: "tk-id",
	};

	function tokClass(capture) {
		if (!capture) return "";
		return tokClassTable[capture.split(".", 1)[0]] || "";
	}

	function computeTokenRanges(lang, source) {
		const info = state.loaded.get(lang);
		if (!info || !info.hasHighlight) return null;
		let res;
		try {
			res = window.gotreesitter.highlight(lang, source);
		} catch (err) {
			res = null;
		}
		if (!res || !res.ok || !Array.isArray(res.ranges)) return null;
		return res.ranges.slice().sort((a, b) => a.startByte - b.startByte || a.endByte - b.endByte);
	}

	// repaint composes the overlay <pre> from three byte-offset channels over
	// the last parsed buffer: syntax token colors, query capture colors
	// (captures win while the query pane is open), and the click-flash pulse.
	// All offsets are into the UTF-8 encoding of renderedSource, so slicing
	// happens on the encoded bytes, not the JS string.
	function repaint() {
		const source = state.renderedSource;
		if (source == null) {
			renderPlain(ui.src.value);
			return;
		}
		const bytes = new TextEncoder().encode(source);
		const n = bytes.length;
		const fg = new Array(n).fill("");
		if (state.tokenRanges) {
			for (const r of state.tokenRanges) {
				const cls = tokClass(r.capture);
				if (!cls || r.endByte <= r.startByte) continue;
				fg.fill(cls, Math.max(0, r.startByte), Math.min(n, r.endByte));
			}
		}
		if (state.queryOpen && state.captures && state.capturesFor === source) {
			for (const c of state.captures.ranges) {
				if (c.end <= c.start) continue;
				fg.fill("pg-capspan pg-cap-" + c.color, Math.max(0, c.start), Math.min(n, c.end));
			}
		}
		const flash = state.flash;
		const inFlash = (i) => !!flash && i >= flash.start && i < flash.end;
		const dec = new TextDecoder();
		const frag = document.createDocumentFragment();
		let i = 0;
		while (i < n) {
			const cls = fg[i];
			const fl = inFlash(i);
			let j = i + 1;
			while (j < n && fg[j] === cls && inFlash(j) === fl) j++;
			const text = dec.decode(bytes.subarray(i, j));
			const full = (cls + (fl ? " pg-flash" : "")).trim();
			if (full) {
				const span = document.createElement("span");
				span.className = full;
				span.textContent = text;
				frag.appendChild(span);
			} else {
				frag.appendChild(document.createTextNode(text));
			}
			i = j;
		}
		frag.appendChild(document.createTextNode("\n"));
		ui.hl.replaceChildren(frag);
		syncScroll();
	}

	let flashTimer = 0;
	function flashSpan(startByte, endByte) {
		state.flash = { start: startByte, end: endByte };
		repaint();
		clearTimeout(flashTimer);
		flashTimer = setTimeout(() => {
			state.flash = null;
			repaint();
		}, 800);
	}

	function syncScroll() {
		ui.hl.scrollTop = ui.src.scrollTop;
		ui.hl.scrollLeft = ui.src.scrollLeft;
	}

	// ---- structured tree pane -------------------------------------------

	// A node is a row when it's named, or the anonymous toggle is on; ERROR
	// and MISSING nodes always show (MISSING is the whole point of the
	// structured bridge — SExpr can't express it).
	function nodeVisible(n) {
		return n.named || state.showAnon || n.missing || n.error;
	}

	const TREE_MAX_LINES = 4000;

	// renderTree walks the structured tree from parse()'s JSON payload.
	// Hidden anonymous nodes don't consume depth — their named descendants
	// render where tree-sitter's named tree would put them. Scroll position
	// survives re-renders. Returns the visible node count (even past the
	// row cap).
	function renderTree() {
		const rootNode = state.tree;
		const frag = document.createDocumentFragment();
		state.visibleRows = [];
		state.activeRowEl = null;
		let total = 0;
		if (!rootNode) {
			const empty = document.createElement("div");
			empty.className = "pg-tree-empty";
			empty.textContent = "(empty tree)";
			frag.appendChild(empty);
		} else {
			const stack = [{ n: rootNode, depth: 0 }];
			while (stack.length) {
				const { n, depth } = stack.pop();
				let childDepth = depth;
				n._el = null;
				if (nodeVisible(n)) {
					total++;
					childDepth = depth + 1;
					if (state.visibleRows.length < TREE_MAX_LINES) {
						const row = document.createElement("div");
						row.className =
							"pg-tline" +
							(n.error ? " pg-err" : "") +
							(n.missing ? " pg-missingrow" : "") +
							(n.named ? "" : " pg-anonrow");
						row.style.paddingLeft = depth * 14 + "px";
						row.dataset.i = String(state.visibleRows.length);
						row.dataset.s16 = String(n.start16);
						row.dataset.e16 = String(n.end16);
						if (n.field) {
							const field = document.createElement("span");
							field.className = "tfield";
							field.textContent = n.field + ": ";
							row.appendChild(field);
						}
						const type = document.createElement("span");
						type.className = "ttype";
						type.textContent = n.named ? n.type : JSON.stringify(n.type);
						row.appendChild(type);
						if (n.missing) {
							const miss = document.createElement("span");
							miss.className = "tmissing";
							miss.textContent = "MISSING";
							row.appendChild(miss);
						}
						n._el = row;
						state.visibleRows.push(n);
						frag.appendChild(row);
					}
				}
				const kids = n.children || [];
				for (let i = kids.length - 1; i >= 0; i--) stack.push({ n: kids[i], depth: childDepth });
			}
			if (total > state.visibleRows.length) {
				const more = document.createElement("div");
				more.className = "pg-tline pg-more";
				more.textContent = "… " + (total - state.visibleRows.length) + " more nodes";
				frag.appendChild(more);
			}
			if (rootNode.truncated) {
				const note = document.createElement("div");
				note.className = "pg-tline pg-more";
				note.textContent = "⚠ tree truncated at 20,000 nodes";
				frag.appendChild(note);
			}
		}
		const scrollTop = ui.tree.scrollTop;
		ui.tree.replaceChildren(frag);
		ui.tree.scrollTop = scrollTop;
		markTreeCaptures();
		return total;
	}

	// deepestVisibleAt returns the deepest visible node whose UTF-16 span
	// contains the caret. Descends through hidden anonymous nodes so their
	// named children stay reachable.
	function deepestVisibleAt(caret) {
		let n = state.tree;
		if (!n || caret < n.start16 || caret > n.end16) return null;
		let best = null;
		for (;;) {
			if (nodeVisible(n) && n.start16 <= caret && caret <= n.end16) best = n;
			const kids = n.children || [];
			let next = null;
			for (const k of kids) {
				if (k.start16 <= caret && caret < k.end16) {
					next = k;
					break;
				}
			}
			if (!next) {
				for (let i = kids.length - 1; i >= 0; i--) {
					const k = kids[i];
					if (k.start16 < k.end16 && k.start16 <= caret && caret === k.end16) {
						next = k;
						break;
					}
				}
			}
			if (!next) break;
			n = next;
		}
		return best;
	}

	function highlightTreeNode(node) {
		if (state.activeRowEl) {
			state.activeRowEl.classList.remove("pg-active");
			state.activeRowEl = null;
		}
		if (!node || !node._el) return;
		node._el.classList.add("pg-active");
		state.activeRowEl = node._el;
		// Scroll the row into view inside the tree pane only (scrollIntoView
		// could also yank the page).
		const cr = ui.tree.getBoundingClientRect();
		const rr = node._el.getBoundingClientRect();
		if (rr.top < cr.top) ui.tree.scrollTop += rr.top - cr.top - 36;
		else if (rr.bottom > cr.bottom) ui.tree.scrollTop += rr.bottom - cr.bottom + 36;
	}

	// selectNodeInEditor: tree row click → textarea selection (UTF-16 span),
	// scrolled into view, plus a flash pulse over the byte span in the overlay.
	function selectNodeInEditor(node) {
		try {
			ui.src.setSelectionRange(node.start16, node.end16);
		} catch (err) {
			return;
		}
		const source = ui.src.value;
		const line = (source.slice(0, node.start16).match(/\n/g) || []).length;
		const lineHeight = parseFloat(getComputedStyle(ui.src).lineHeight) || 23;
		ui.src.scrollTop = Math.max(0, line * lineHeight - ui.src.clientHeight * 0.35);
		const col = node.start16 - (source.lastIndexOf("\n", node.start16 - 1) + 1);
		ui.src.scrollLeft = Math.max(0, col * 8.1 - ui.src.clientWidth * 0.3);
		syncScroll();
		flashSpan(node.start, node.end);
	}

	// ---- query pane -------------------------------------------------------

	// Deterministic per-capture-name palette assignment (djb2 → 8 colors).
	function capColorIdx(name) {
		let h = 5381;
		for (let i = 0; i < name.length; i++) h = ((h * 33) ^ name.charCodeAt(i)) >>> 0;
		return h % 8;
	}

	function clearQueryError() {
		ui.qerr.hidden = true;
		ui.qerr.replaceChildren();
	}

	// showQueryError renders the compile error, and when the bridge extracted
	// an offset from the engine message, an excerpt of the offending query
	// line with a caret under the position.
	function showQueryError(message, offset, keptStale) {
		ui.qerr.hidden = false;
		ui.qerr.replaceChildren();
		const msg = document.createElement("div");
		msg.className = "pg-qerrmsg";
		msg.textContent = message;
		ui.qerr.appendChild(msg);
		if (typeof offset === "number" && offset >= 0) {
			const q = ui.query.value;
			// offset is a byte offset into the query text; align it to JS
			// string indexes when the query contains non-ASCII.
			let charOff = q.length;
			let bi = 0;
			let ci = 0;
			const enc = new TextEncoder();
			for (const ch of q) {
				if (bi >= offset) {
					charOff = ci;
					break;
				}
				bi += enc.encode(ch).length;
				ci += ch.length;
			}
			if (bi >= offset && charOff === q.length && ci <= q.length) charOff = Math.min(ci, q.length);
			const lineStart = q.lastIndexOf("\n", Math.max(0, charOff - 1)) + 1;
			let lineEnd = q.indexOf("\n", charOff);
			if (lineEnd < 0) lineEnd = q.length;
			const line = document.createElement("div");
			line.className = "pg-qerrline";
			line.textContent = q.slice(lineStart, lineEnd);
			const caret = document.createElement("div");
			caret.className = "pg-qerrcaret";
			caret.textContent = " ".repeat(Math.max(0, charOff - lineStart)) + "^";
			ui.qerr.appendChild(line);
			ui.qerr.appendChild(caret);
		}
		if (keptStale) {
			const note = document.createElement("div");
			note.className = "pg-qerrnote";
			note.textContent = "captures from the last valid query are still shown";
			ui.qerr.appendChild(note);
		}
	}

	function renderLegend(caps) {
		if (!caps || !caps.byName.size) {
			ui.legend.hidden = true;
			ui.legend.replaceChildren();
			return;
		}
		ui.legend.hidden = false;
		const frag = document.createDocumentFragment();
		for (const [name, info] of caps.byName) {
			const item = document.createElement("span");
			item.className = "pg-legenditem";
			const swatch = document.createElement("i");
			swatch.className = "pg-swatch pg-swatch-" + info.color;
			item.appendChild(swatch);
			const label = document.createElement("span");
			label.textContent = "@" + name + " ×" + info.count;
			item.appendChild(label);
			frag.appendChild(item);
		}
		if (caps.truncated) {
			const trunc = document.createElement("span");
			trunc.className = "pg-legendtrunc";
			trunc.textContent = "truncated at 500 matches";
			frag.appendChild(trunc);
		}
		ui.legend.replaceChildren(frag);
	}

	// markTreeCaptures decorates tree rows whose exact node (byte span +
	// type) was captured by the current query: capture-name badges with the
	// legend's colors.
	function markTreeCaptures() {
		ui.tree.querySelectorAll(".pg-capbadge").forEach((el) => el.remove());
		ui.tree.querySelectorAll(".pg-tline.pg-capped").forEach((el) => el.classList.remove("pg-capped"));
		if (!state.queryOpen || !state.captures || state.capturesFor !== state.renderedSource) return;
		const marks = state.captures.nodeMarks;
		if (!marks.size) return;
		for (const node of state.visibleRows) {
			if (!node._el) continue;
			const arr = marks.get(node.start + ":" + node.end + ":" + node.type);
			if (!arr) continue;
			node._el.classList.add("pg-capped");
			for (const m of arr) {
				const badge = document.createElement("span");
				badge.className = "pg-capbadge";
				const swatch = document.createElement("i");
				swatch.className = "pg-swatch pg-swatch-" + m.color;
				badge.appendChild(swatch);
				badge.appendChild(document.createTextNode("@" + m.name));
				node._el.appendChild(badge);
			}
		}
	}

	// runQuery compiles + executes the query pane's text against the last
	// parsed buffer (so spans always match the rendered tree/overlay).
	// Invalid queries show the error but keep the last good captures.
	function runQuery() {
		if (!state.ready || !state.queryOpen) return;
		const qtext = ui.query.value;
		if (!qtext.trim()) {
			state.captures = null;
			state.capturesFor = null;
			clearQueryError();
			renderLegend(null);
			markTreeCaptures();
			repaint();
			return;
		}
		const lang = state.renderedLang;
		const source = state.renderedSource;
		if (!lang || source == null) return;
		let res;
		try {
			res = window.gotreesitter.query(lang, source, qtext);
		} catch (err) {
			res = { ok: false, error: String(err) };
		}
		if (!res || !res.ok) {
			// Keep the last good captures visible — but only if they still
			// belong to the buffer on screen; a stale buffer's offsets would
			// paint garbage.
			if (state.captures && state.capturesFor !== source) {
				state.captures = null;
				state.capturesFor = null;
				renderLegend(null);
				markTreeCaptures();
				repaint();
			}
			showQueryError(
				(res && res.error) || "query failed",
				res && typeof res.errorOffset === "number" ? res.errorOffset : undefined,
				!!(state.captures && state.captures.byName.size)
			);
			return;
		}
		clearQueryError();
		const byName = new Map(); // insertion order = first appearance
		const ranges = [];
		const nodeMarks = new Map();
		const matches = Array.isArray(res.matches) ? res.matches : [];
		for (const m of matches) {
			for (const c of m.captures || []) {
				let entry = byName.get(c.name);
				if (!entry) {
					entry = { color: capColorIdx(c.name), count: 0 };
					byName.set(c.name, entry);
				}
				entry.count++;
				ranges.push({ start: c.start, end: c.end, color: entry.color });
				const key = c.start + ":" + c.end + ":" + (c.type || "");
				let arr = nodeMarks.get(key);
				if (!arr) {
					arr = [];
					nodeMarks.set(key, arr);
				}
				if (!arr.some((x) => x.name === c.name)) arr.push({ name: c.name, color: entry.color });
			}
		}
		ranges.sort((a, b) => a.start - b.start || a.end - b.end);
		state.captures = { ranges, byName, nodeMarks, truncated: !!res.truncated };
		state.capturesFor = source;
		renderLegend(state.captures);
		markTreeCaptures();
		repaint();
	}

	let queryTimer = 0;
	function scheduleQuery() {
		clearTimeout(queryTimer);
		queryTimer = setTimeout(runQuery, 250);
	}

	// ---- auto-detect ----------------------------------------------------

	// Instant, zero-cost signatures. First hit wins; order matters (the
	// specific before the generic — markdown last since # comments and
	// fences overlap other languages).
	function heuristicDetect(source) {
		const head = source.slice(0, 4096);
		const firstLine = head.slice(0, head.indexOf("\n") + 1 || head.length);
		if (firstLine.startsWith("#!")) {
			if (/python/.test(firstLine)) return "python";
			if (/node/.test(firstLine)) return "javascript";
			if (/ruby/.test(firstLine)) return "ruby";
			if (/(ba|z)?sh/.test(firstLine)) return "bash";
		}
		if (/^\s*<\?php/.test(head)) return "php";
		if (/^\s*(<!DOCTYPE|<html)/i.test(head)) return "html";
		if (/^\s*[{[]/.test(head) && /"[^"\n]*"\s*:/.test(head)) return "json";
		if (/\bpackage\s+\w+/.test(head) && /\bfunc\b/.test(head)) return "go";
		if (/#include\s*[<"]/.test(head)) return "c";
		if (/\bfn\s+\w+\s*\(/.test(head) && (/\blet\s+mut\b/.test(head) || /::/.test(head) || /->/.test(head))) return "rust";
		if (/^\s*def\s+\w+\s*\(.*\)\s*:/m.test(head)) return "python";
		if (/^\s*(SELECT|INSERT|UPDATE|DELETE|CREATE)\s/im.test(head) && /\b(FROM|INTO|TABLE|SET|WHERE)\b/i.test(head)) return "sql";
		if (/^```/m.test(head) || /^#{1,6}\s+\S/m.test(head)) return "markdown";
		return null;
	}

	let detectTimer = 0;
	function scheduleDetect() {
		if (!state.auto) return;
		clearTimeout(detectTimer);
		detectTimer = setTimeout(runDetect, 600);
	}

	async function runDetect() {
		if (!state.auto || !state.ready) return;
		const source = ui.src.value;
		if (!source.trim()) return;
		const seq = ++state.detectSeq;
		let data;
		try {
			const resp = await fetch("/playground/detect", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ source: source.slice(0, 4096) }),
			});
			if (!resp.ok) return;
			data = await resp.json();
		} catch (err) {
			return; // detection is advisory; never break the editor over it
		}
		if (seq !== state.detectSeq || !state.auto || !data || !data.ok || !data.best) return;
		const confidence = typeof data.confidence === "number" ? data.confidence : 0;
		const label = confidence >= 0.75 ? "high" : confidence >= 0.5 ? "medium" : "low";
		setBadge("detected " + data.best + " · " + label + " " + Math.round(confidence * 100) + "%");
		if (data.best !== state.detected && confidence >= 0.5) {
			state.detected = data.best;
			scheduleParse(0);
		}
	}

	// ---- parse loop -----------------------------------------------------

	let parseTimer = 0;
	function scheduleParse(delay) {
		clearTimeout(parseTimer);
		parseTimer = setTimeout(runParse, delay == null ? 200 : delay);
	}

	async function runParse() {
		if (!state.ready) return;
		const seq = ++state.parseSeq;
		let lang;
		if (state.auto) {
			const guess = heuristicDetect(ui.src.value);
			if (guess && guess !== state.detected) {
				state.detected = guess;
				setBadge("detected " + guess + " · signature");
			}
			lang = state.detected;
		} else {
			lang = state.manualLang;
		}
		if (!lang) return;
		try {
			await ensureLanguage(lang);
		} catch (err) {
			setStat(String(err && err.message ? err.message : err), "err");
			return;
		}
		if (seq !== state.parseSeq) return; // superseded while the blob loaded
		const source = ui.src.value; // freshest buffer
		const t0 = performance.now();
		let res;
		try {
			res = window.gotreesitter.parse(lang, source);
		} catch (err) {
			res = { ok: false, error: String(err) };
		}
		const elapsed = performance.now() - t0;
		if (!res || !res.ok) {
			setStat("parse failed: " + ((res && res.error) || "unknown error"), "err");
			return;
		}
		state.activeLang = lang;
		ui.langtag.textContent = lang + (state.auto ? " · auto" : "");
		let treeObj = null;
		try {
			treeObj = res.tree ? JSON.parse(res.tree) : null;
		} catch (err) {
			treeObj = null;
		}
		state.tree = treeObj;
		state.renderedSource = source;
		state.renderedLang = lang;
		state.tokenRanges = computeTokenRanges(lang, source);
		state.flash = null;
		state.lastElapsed = elapsed;
		state.lastDot = res.hasError ? "err" : "ok";
		const nodes = renderTree();
		repaint(); // tokens first; a valid query below repaints with captures
		if (state.queryOpen && ui.query.value.trim()) {
			runQuery(); // refresh captures against the new buffer
		}
		setStat(elapsed.toFixed(1) + " ms · " + nodes + " nodes", state.lastDot);
	}

	// ---- events ---------------------------------------------------------

	ui.src.addEventListener("input", () => {
		// The overlay must mirror the buffer immediately; every coordinate
		// we hold is stale until the next parse lands.
		state.renderedSource = null;
		state.flash = null;
		renderPlain(ui.src.value);
		scheduleParse(200);
		scheduleDetect();
	});
	ui.src.addEventListener("scroll", syncScroll);

	// Caret/selection → tree sync (debounced; selectionchange only fires on
	// the document, so gate on focus; keyup/click cover engines that don't
	// emit selectionchange for textareas).
	let selTimer = 0;
	function scheduleCaretSync() {
		clearTimeout(selTimer);
		selTimer = setTimeout(() => {
			if (document.activeElement !== ui.src || state.renderedSource == null) return;
			highlightTreeNode(deepestVisibleAt(ui.src.selectionStart));
		}, 120);
	}
	document.addEventListener("selectionchange", () => {
		if (document.activeElement === ui.src) scheduleCaretSync();
	});
	ui.src.addEventListener("keyup", scheduleCaretSync);
	ui.src.addEventListener("click", scheduleCaretSync);

	// Tree row → editor selection sync.
	ui.tree.addEventListener("click", (e) => {
		const row = e.target.closest(".pg-tline[data-i]");
		if (!row) return;
		const node = state.visibleRows[parseInt(row.dataset.i, 10)];
		if (!node) return;
		selectNodeInEditor(node);
		highlightTreeNode(node);
	});

	ui.anon.addEventListener("change", () => {
		state.showAnon = ui.anon.checked;
		const nodes = renderTree();
		if (state.lastElapsed != null && state.renderedSource != null) {
			setStat(state.lastElapsed.toFixed(1) + " ms · " + nodes + " nodes", state.lastDot);
		}
	});

	ui.query.addEventListener("input", scheduleQuery);

	ui.qhint.addEventListener("click", () => {
		ui.query.value = '((identifier) @id (#match? @id "^ma"))';
		runQuery();
		ui.query.focus();
	});

	ui.qcollapse.addEventListener("click", () => {
		state.queryOpen = !state.queryOpen;
		ui.qbd.hidden = !state.queryOpen;
		ui.qcollapse.textContent = state.queryOpen ? "hide" : "show";
		ui.qcollapse.setAttribute("aria-expanded", String(state.queryOpen));
		if (state.queryOpen) {
			runQuery();
		} else {
			// Captures only win while the pane is open — restore pure
			// syntax highlighting and unmark the tree.
			markTreeCaptures();
			repaint();
		}
	});

	ui.lang.addEventListener("change", () => {
		const value = ui.lang.value;
		if (value === "") {
			state.auto = true;
			state.manualLang = null;
			scheduleParse(0);
			scheduleDetect();
		} else {
			state.auto = false;
			state.manualLang = value;
			state.detectSeq++; // cancel any in-flight race
			setBadge("");
			scheduleParse(0);
		}
	});

	root.querySelectorAll(".qchip[data-sample]").forEach((chip) => {
		chip.addEventListener("click", () => {
			const sample = samples[chip.dataset.sample];
			if (sample == null) return;
			ui.src.value = sample;
			ui.src.scrollTop = 0;
			ui.src.scrollLeft = 0;
			state.renderedSource = null;
			state.flash = null;
			renderPlain(sample);
			if (state.auto) {
				setBadge("");
				scheduleDetect();
			}
			scheduleParse(0);
			if (!ui.src.disabled) ui.src.focus();
		});
	});

	renderPlain(ui.src.value);
	boot().catch((err) => failHard(String(err && err.message ? err.message : err)));
})();
