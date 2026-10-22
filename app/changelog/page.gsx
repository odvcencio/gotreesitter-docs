package changelog

func Page() Node {
	return <section class="change-page">
		<header class="change-hero">
			<div class="change-hero-copy">
				<span class="eyebrow">Release archive</span>
				<h1 class="change-title">History you can interrogate.</h1>
				<p class="change-lead">
					Search every shipped release and the pinned source snapshot. Follow each claim to its tag, code comparison, pull request, issue, or source line.
				</p>
				<div class="change-source-seal" aria-label="Changelog source evidence">
					<span class="change-seal-label">source snapshot</span>
					<a href={data.sourceURL} target="_blank" rel="noopener noreferrer">
						commit
						{data.sourceCommit}
					</a>
					<span aria-hidden="true">·</span>
					<span>SHA-256 {data.sourceSHA256}</span>
				</div>
			</div>
			<aside class="change-latest" aria-label="Latest release">
				<span class="change-latest-kicker">latest immutable release</span>
				<strong>{data.latestVersion}</strong>
				<p>{data.latestSummary}</p>
				<a href={data.latestReleaseURL} target="_blank" rel="noopener noreferrer">Read the release notes ↗</a>
			</aside>
		</header>

		<div class="change-stats" aria-label="Archive summary">
			<div class="change-stat">
				<span class="change-stat-value">{data.releaseCount}</span>
				<span class="change-stat-label">immutable releases</span>
			</div>
			<div class="change-stat">
				<span class="change-stat-value">{data.totalEntries}</span>
				<span class="change-stat-label">source-backed entries</span>
			</div>
			<div class="change-stat">
				<span class="change-stat-value">{data.currentEntries}</span>
				<span class="change-stat-label">current entries on main</span>
			</div>
			<div class="change-stat">
				<span class="change-stat-value">{data.earliestVersion}</span>
				<span class="change-stat-label">archive begins {data.earliestDate}</span>
			</div>
		</div>

		<section class="change-thread" aria-labelledby="campaign-thread-title">
			<div class="change-thread-head">
				<div>
					<span class="change-kicker">release thread</span>
					<h2 id="campaign-thread-title">From recovery repair to parser ownership.</h2>
				</div>
				<p>
					The {data.latestVersion} tag records the release scope. The source snapshot preserves each release and its evidence.
				</p>
			</div>
			<ol class="change-thread-list">
				<Each as="step" of={data.campaignTrail}>
					<li>
						<a href={step.href} target="_blank" rel="noopener noreferrer">{step.label}</a>
						<span>{step.description}</span>
					</li>
				</Each>
			</ol>
		</section>

		<section class="change-explorer" aria-labelledby="change-explorer-title">
			<div class="change-explorer-head">
				<div>
					<span class="change-kicker">explore the record</span>
					<h2 id="change-explorer-title">Find the change that affects you.</h2>
				</div>
				<p>Filters submit as ordinary GET requests. Results remain linkable, server-rendered, and usable without client code.</p>
			</div>
			{data.filterForm}
			<div class="change-results-bar" aria-live="polite">
				<strong>{data.resultCount} matching changes</strong>
				<If when={data.hasFilters}>
					<span>Filtered from {data.totalEntries} entries.</span>
				</If>
				<If when={data.hasFilters == false}>
					<span>Showing released history and current work.</span>
				</If>
			</div>
		</section>

		<If when={data.hasResults == false}>
			<div class="change-empty" role="status">
				<span aria-hidden="true">∅</span>
				<h2>No source entry matches these filters.</h2>
				<p>Try a broader term, another category, or both release states.</p>
				<a href="/changelog" data-gosx-link="true">Reset the explorer</a>
			</div>
		</If>

		<div class="change-index">
			<Each as="group" of={data.groups}>
				<section class="change-group" id={group.id} aria-labelledby={group.id + "-title"}>
					<h2 class="change-group-title" id={group.id + "-title"}>
						<a href={group.href} data-gosx-link="true">{group.label}</a>
						<span class="change-group-count">{group.releaseCount}</span>
					</h2>
					<ol class="change-index-list">
						<Each as="row" of={group.rows}>
							<li class="change-index-row" id={row.id}>
								<span class="change-anchor" id={row.altID}></span>
								<a class="change-index-version" href={row.href} data-gosx-link="true">{row.version}</a>
								<span class="change-index-date">{row.date}</span>
								<span class={"change-impact " + row.impactClass}>{row.impact}</span>
								<span class="change-index-summary">{row.summary}</span>
								<span class="change-count">{row.entryCount} changes</span>
							</li>
						</Each>
					</ol>
				</section>
			</Each>
		</div>

		<footer class="change-foot">
			<p>
				Released sections describe immutable tags. Unreleased sections describe the pinned main snapshot and can change before the next tag.
			</p>
			<a href={data.sourceURL} target="_blank" rel="noopener noreferrer">Audit the source snapshot ↗</a>
		</footer>
	</section>
}
