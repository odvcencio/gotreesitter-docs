package docs

func Page() Node {
	return <article class="prose">
		{data.content}
	</article>
}
