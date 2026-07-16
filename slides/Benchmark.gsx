package main

// Benchmark is the one live data surface in the talk. It lets the presenter
// collapse the GLR forks and reveal the measured 5.95x -> 4.41x wall-time move.
//
//gosx:island
func Benchmark(props any) Node {
	collapsed := signal.New(false)
	toggle := func() { collapsed.Set(!collapsed.Get()) }

	return <div class="bench">
		<div class="bench-row baseline">
			<span class="bench-name">tree-sitter C</span>
			<div class="bench-track"><div class="bench-fill c">1.00x</div></div>
		</div>
		<div class="bench-row">
			<span class="bench-name">Go · before</span>
			<div class="bench-track"><div class="bench-fill before">5.95x</div></div>
		</div>
		<div class="bench-row">
			<span class="bench-name">Go · fork collapse</span>
			<div class="bench-track"><div class={"bench-fill after " + (collapsed.Get() ? "is-live" : "")}>4.41x</div></div>
		</div>
		<div class="bench-action">
			<span>{collapsed.Get() ? "−26% wall · byte-for-byte parity held" : "C-deterministic forks are still live"}</span>
			<button onClick={toggle}>{collapsed.Get() ? "Restore forks" : "Collapse forks"}</button>
		</div>
	</div>
}
