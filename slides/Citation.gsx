package main

// Citation keeps quantitative claims attached to a public, rerunnable receipt.
//
//gosx:island
func Citation(props any) Node {
	return <a class="citation" href={props.href} target="_blank" rel="noopener noreferrer">
		<span>◆</span>
		{props.label}
	</a>
}
