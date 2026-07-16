package docs

type LangSearchItem struct {
	Name  string
	Color string
	TS    bool
}

type LangSearchProps struct {
	Langs []LangSearchItem
}

// LangSearch is authored and compiled as a normal GoSX island. The language
// data still comes from the Markdown langlist block; only the local filter
// state lives in the browser.
//
//gosx:island
func LangSearch(props LangSearchProps) Node {
	query := signal.New("")
	setQuery := func() { query.Set(value) }

	return <div class="lang-island">
		<div class="langbar">
			<input
				class="search mono"
				type="text"
				placeholder="filter languages…  try 'script' or 'go'"
				aria-label="Filter languages"
				onInput={setQuery}
			 />
			<span class="count mono">
				{langs.filter(func(lang){ return lang.name.toLower().contains(query.Get().toLower()) }).length}
				/
				{langs.length}
			</span>
		</div>
		<div class="langgrid">
			<Each of={langs} as="lang">
				<div class={lang.name.toLower().contains(query.Get().toLower()) ? "langtile" : "langtile hidden"}>
					<span class={"ldot " + lang.color}></span>
					<span class="lname">{lang.name}</span>
					<If when={lang.ts}>
						<span class="lts">TS</span>
					</If>
				</div>
			</Each>
		</div>
	</div>
}
