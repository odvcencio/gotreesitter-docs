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
				<p>Version 0.48 adds Swift corpus coverage, route evidence, and recovery corrections.</p>
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
					The v0.48 tag records the campaign results. New main entries remain separate from immutable release evidence.
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

		<details class="change-version-index">
			<summary>Jump to any version</summary>
			<nav aria-label="Changelog versions">
				<Each as="version" of={data.versionLinks}>
					<a class={"change-version-link " + version.status} href={version.href}>{version.version}</a>
				</Each>
			</nav>
		</details>

		<If when={data.hasResults == false}>
			<div class="change-empty" role="status">
				<span aria-hidden="true">∅</span>
				<h2>No source entry matches these filters.</h2>
				<p>Try a broader term, another category, or both release states.</p>
				<a href="/changelog" data-gosx-link="true">Reset the explorer</a>
			</div>
		</If>

		<div class="change-release-list">
			<Each as="release" of={data.releases}>
				<details
					class={"change-release " + release.status}
					id={release.id}
					open={release.open}
				>
					<summary class="change-release-summary">
						<span class="change-release-marker" aria-hidden="true"></span>
						<span class="change-release-identity">
							<strong>{release.version}</strong>
							<span>{release.date}</span>
						</span>
						<span class={"change-release-status " + release.status}>{release.statusLabel}</span>
						<span class={"change-impact " + release.impactClass}>{release.impact}</span>
						<span class="change-count">{release.entryCount} changes</span>
						<span class="change-chevron" aria-hidden="true">↓</span>
					</summary>
					<div class="change-release-body">
						<If when={release.hasNarrative}>
							<div class="change-narrative">
								<span class="change-kicker">release narrative</span>
								<h3>{release.narrativeTitle}</h3>
								<p>{release.narrativeBody}</p>
							</div>
						</If>

						<div class="change-evidence-links" aria-label="Release evidence">
							<a href={release.evidenceURL} target="_blank" rel="noopener noreferrer">Release evidence ↗</a>
							<a href={release.codeURL} target="_blank" rel="noopener noreferrer">Code changes ↗</a>
							<a href={release.sourceURL} target="_blank" rel="noopener noreferrer">Changelog source ↗</a>
						</div>

						<If when={release.hasTrail}>
							<div class="change-related">
								<span class="change-related-title">Related trail</span>
								<div>
									<Each as="trail" of={release.historicalTrail}>
										<a href={trail.href} target="_blank" rel="noopener noreferrer">
											<strong>{trail.label}</strong>
											<span>{trail.description}</span>
										</a>
									</Each>
								</div>
							</div>
						</If>

						<div class="change-sections">
							<Each as="section" of={release.sections}>
								<section class="change-section" id={section.id}>
									<header class="change-section-head">
										<span class={"change-section-dot " + section.color} aria-hidden="true"></span>
										<div>
											<h3>{section.name}</h3>
											<span>{section.impact}</span>
										</div>
										<a href={section.sourceURL} target="_blank" rel="noopener noreferrer">source ↗</a>
										<span class="change-section-count">{section.entryCount}</span>
									</header>
									<ol class="change-entry-list">
										<Each as="entry" of={section.entries}>
											<li class="change-entry">
												<div class="change-entry-copy">{entry.content}</div>
												<div class="change-entry-evidence">
													<a
														class="change-source-link"
														href={entry.sourceURL}
														target="_blank"
														rel="noopener noreferrer"
													>{entry.sourceLabel} ↗</a>
													<If when={entry.hasRefs}>
														<span class="change-entry-refs">
															<Each as="reference" of={entry.references}>
																<a
																	href={reference.href}
																	target="_blank"
																	rel="noopener noreferrer"
																>{reference.label}</a>
															</Each>
														</span>
													</If>
												</div>
											</li>
										</Each>
									</ol>
								</section>
							</Each>
						</div>

						<nav class="change-adjacent" aria-label="Adjacent versions">
							<If when={release.hasPrevious}>
								<a href={release.previous.href}>← Older: {release.previous.version}</a>
							</If>
							<If when={release.hasNext}>
								<a href={release.next.href}>Newer: {release.next.version} →</a>
							</If>
						</nav>
					</div>
				</details>
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
