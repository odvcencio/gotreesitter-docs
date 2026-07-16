package main

// GLRForks is the talk's mental model for GLR parsing: step through a real
// ambiguity (`async ( x ) => { … }` — call expression vs arrow parameters)
// and watch the stack FORK at the conflict, both forks advance in lockstep,
// the wrong fork DIE at `=>`, and the second would-be fork at `{` never
// happen because the C oracle proves that state deterministic (PR #90).
// All geometry is pre-drawn; the step signal swaps a class on the root and
// deck.css transitions each phase in.
//
//gosx:island
func GLRForks(props any) Node {
	step := signal.New(0)
	next := func() { step.Set(step.Get() < 5 ? step.Get() + 1 : 0) }
	back := func() { step.Set(step.Get() > 0 ? step.Get() - 1 : 0) }

	return <div class={"glrforks " + (step.Get() == 0 ? "s0" : step.Get() == 1 ? "s1" : step.Get() == 2 ? "s2" : step.Get() == 3 ? "s3" : step.Get() == 4 ? "s4" : "s5")}>
		<div class="glr-tokens">
			<span class="glr-tok tk0">async</span>
			<span class="glr-tok tk1">(</span>
			<span class="glr-tok tk2">x</span>
			<span class="glr-tok tk3">)</span>
			<span class="glr-tok tk4">=&gt;</span>
			<span class="glr-tok tk5">&#123;</span>
			<span class="glr-tok tk6">…</span>
		</div>
		<svg class="glr-svg" viewBox="0 0 1520 430" preserveAspectRatio="xMidYMid meet">
			<line class="glr-edge from-0 e-main1" x1="120" y1="140" x2="280" y2="140"></line>
			<circle class="glr-node from-0 n0" cx="100" cy="140" r="20"></circle>
			<circle class="glr-node from-0 n1" cx="300" cy="140" r="20"></circle>

			<line class="glr-edge glr-fork from-1 e-forkA" x1="320" y1="140" x2="500" y2="80"></line>
			<line class="glr-edge glr-fork from-1 e-forkB" x1="320" y1="140" x2="500" y2="240"></line>
			<circle class="glr-node glr-a from-1 nA1" cx="520" cy="80" r="20"></circle>
			<circle class="glr-node glr-b from-1 nB1" cx="520" cy="240" r="20"></circle>
			<text class="glr-lab glr-a from-1" x="560" y="52">call expression?</text>
			<text class="glr-lab glr-b from-1" x="560" y="296">arrow parameters?</text>

			<line class="glr-edge glr-a from-2 eA2" x1="540" y1="80" x2="700" y2="80"></line>
			<line class="glr-edge glr-a from-2 eA3" x1="740" y1="80" x2="880" y2="80"></line>
			<circle class="glr-node glr-a from-2 nA2" cx="720" cy="80" r="20"></circle>
			<circle class="glr-node glr-a from-2 nA3" cx="900" cy="80" r="20"></circle>
			<line class="glr-edge glr-b from-2 eB2" x1="540" y1="240" x2="700" y2="240"></line>
			<line class="glr-edge glr-b from-2 eB3" x1="740" y1="240" x2="880" y2="240"></line>
			<circle class="glr-node glr-b from-2 nB2" cx="720" cy="240" r="20"></circle>
			<circle class="glr-node glr-b from-2 nB3" cx="900" cy="240" r="20"></circle>

			<text class="glr-die from-3" x="954" y="88">✕ dies at =&gt;</text>

			<line class="glr-edge glr-b from-3 eB4" x1="920" y1="240" x2="1080" y2="240"></line>
			<circle class="glr-node glr-b from-3 nB4" cx="1100" cy="240" r="20"></circle>

			<line class="glr-edge glr-ghost from-4 eG1" x1="1120" y1="240" x2="1240" y2="180"></line>
			<line class="glr-edge glr-ghost from-4 eG2" x1="1120" y1="240" x2="1240" y2="300"></line>
			<circle class="glr-node glr-ghost from-4 nG1" cx="1250" cy="180" r="16"></circle>
			<circle class="glr-node glr-ghost from-4 nG2" cx="1250" cy="300" r="16"></circle>
			<text class="glr-lab glr-det from-4" x="1500" y="130">tables say: deterministic — no fork</text>

			<line class="glr-edge glr-b from-4 eB5" x1="1120" y1="240" x2="1300" y2="240"></line>
			<circle class="glr-node glr-b from-4 nB5" cx="1320" cy="240" r="20"></circle>
			<line class="glr-edge glr-win from-5 eB6" x1="1340" y1="240" x2="1440" y2="240"></line>
			<circle class="glr-node glr-win from-5 nB6" cx="1460" cy="240" r="20"></circle>
		</svg>
		<div class="glr-foot">
			<span class="glr-cap">{step.Get() == 0 ? "One stack. The prefix is deterministic — LR speed, nothing speculative." : step.Get() == 1 ? "Conflict at ( — the table has two actions, so the stack FORKS: call expression ∥ arrow parameters." : step.Get() == 2 ? "Both forks advance in lockstep, token by token. No backtracking — ambiguity is carried forward." : step.Get() == 3 ? "=> can't follow a finished call: that fork hits a dead end and dies. No error, no rewind." : step.Get() == 4 ? "At { the table LOOKS ambiguous — but the C oracle proves this state deterministic, so no fork is created (PR #90)." : "One stack survives: the exact tree C selects, at 4.41x instead of 5.95x wall."}</span>
			<span class="glr-btns">
				<button class="glr-btn" onClick={back}>‹ back</button>
				<button class="glr-btn" onClick={next}>step ›</button>
			</span>
		</div>
	</div>
}
