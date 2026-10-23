package series

func Page() Node {
	return <section class="change-page">
		<header class="change-series-head">
			<a class="change-back" href={data.indexHref} data-gosx-link="true">← All releases</a>
			<span class="eyebrow">Release series</span>
			<h1 class="change-title">{data.label}</h1>
			<p class="change-lead">Every release in this series, with its entries, evidence links and source lines.</p>
			<If when={data.hasFilters}>
				<p class="change-results-bar" aria-live="polite"><strong>{data.resultCount} matching changes</strong> in this series.</p>
			</If>
		</header>

		<If when={data.hasResults == false}>
			<div class="change-empty" role="status">
				<span aria-hidden="true">∅</span>
				<h2>No entry in this series matches the filters.</h2>
				<a href={data.indexHref} data-gosx-link="true">Back to all releases</a>
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
								<h2>{release.narrativeTitle}</h2>
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
											<h2>{section.name}</h2>
											<span>{section.impact}</span>
										</div>
										<a href={section.sourceURL} target="_blank" rel="noopener noreferrer">source ↗</a>
										<span class="change-section-count">{section.entryCount}</span>
									</header>
									<If when={section.hasIntroduction}>
										<div class="change-entry-copy">{section.introduction}</div>
									</If>
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

		<nav class="change-adjacent change-series-nav" aria-label="Adjacent series">
			<If when={data.hasOlder}>
				<a href={data.olderHref} data-gosx-link="true">← Older: {data.olderLabel}</a>
			</If>
			<If when={data.hasNewer}>
				<a href={data.newerHref} data-gosx-link="true">Newer: {data.newerLabel} →</a>
			</If>
		</nav>

		<footer class="change-foot">
			<p>
				Released sections describe immutable tags. Unreleased sections describe the pinned main snapshot and can change before the next tag.
			</p>
			<a href={data.sourceURL} target="_blank" rel="noopener noreferrer">Audit the source snapshot ↗</a>
		</footer>
	</section>
}
