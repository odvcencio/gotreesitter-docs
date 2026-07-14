package docs

func Page() Node {
	return <section class="page">
		{data.contentBefore}
		<If when={data.hasLangSearch}>
			<Surface
				name="GoTreesitterLangSearch"
				runtime="go-wasm"
				wasmPath={data.langSearchWasmURL}
				capabilities="wasm text-input"
				requiredCapabilities="wasm"
			>
				<LangSearch langs={data.langs} tokenSources={data.langTokenSources} total={data.langTotal} />
			</Surface>
		</If>
		{data.contentAfter}
	</section>
}

// LangSearch is the complete server-rendered language catalog. A dedicated
// standard-Go GoSX engine enhances this fallback with client-side filtering.
func LangSearch(props any) Node {
	return <div class="lang-island">
		<div class="langbar">
			<input
				class="search mono"
				type="text"
				placeholder="filter 206 languages…  try 'script' or 'go'"
				aria-label="Filter languages"
			/>
			<span class="count mono">
				{props.Total} / {props.Total}
			</span>
		</div>
		<div class="langgrid">
			<Each of={props.Langs} as="l">
				<div class="langtile">
					<span class="ldot"></span>
					<span class="lname">{l}</span>
					<If when={props.TokenSources.contains(l)}>
						<span class="lts">TS</span>
					</If>
				</div>
			</Each>
		</div>
	</div>
}
