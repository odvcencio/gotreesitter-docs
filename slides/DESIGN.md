# GoTreeSitter deck design contract

This contract is derived from the supplied “GopherCon GoTreeSitter Deck v2”
artifact and governs `deck.md`, the GoSX islands, and `deck.css` (auto-loaded
by gosx-slides after the base theme).

## Visual System

### Territory

**Paper & Ink.** Warm paper fills the canvas, near-black ink provides the
structure, and hard-edged cards feel printed and assembled. The signature move
is the recurring ink outline plus offset print shadow, interrupted by small
high-chroma registration colors. Avoid soft gradients, glass effects, and
generic rounded SaaS cards.

### Typography

- Display: **Space Grotesk 700**, tight tracking and 1.02 leading.
- Body: **Space Grotesk 400/500/600**.
- Mono: **JetBrains Mono 400/600/700** for evidence, code, labels, and chrome.
- Scale: **1.333 (Perfect Fourth)**, expressed through responsive `--type-*`
  tokens from `--type-xs` to `--type-hero`.

### Color architecture

- Dominant 60%: paper `#efe9db`.
- Secondary 30%: card `#fbf7ee`, paper-2 `#e7e0cf`, and ink `#141210` section
  dividers.
- Accent 10%: pink `#ff5da2` by default, with yellow, cyan, green, blue, violet,
  orange, and red used as registration marks and evidence colors.
- Primary ink `#141210` on paper: **15.44:1, WCAG AAA**.
- Body ink `#2a2620` on paper: **12.42:1, WCAG AAA**.
- Muted ink `#726b5c` on paper: **4.37:1, WCAG AA for large text**; it is
  restricted to large mono chrome.
- Paper text `#efe9db` on ink: **15.44:1, WCAG AAA**.
- Dark-slide body `#d8d2c2` on ink: **12.38:1, WCAG AAA**.
- Dark-slide muted `#a89f8d` on ink: **7.13:1, WCAG AAA**.

### Motion

**Subtle.** Slide entry is 220 ms; progress and color changes are 260–300 ms;
the benchmark bar settles over 600 ms. `--ease-out` is
`cubic-bezier(0.25, 1, 0.5, 1)` and `--ease-spring` is
`cubic-bezier(0.34, 1.56, 0.64, 1)`. Motion is removed under
`prefers-reduced-motion`.

### Spacing

The scale uses an 8 px base and responsive page gutters:

- `--space-xs: 0.75rem`
- `--space-sm: 1rem`
- `--space-md: 1.5rem`
- `--space-lg: 2rem`
- `--space-xl: 3rem`
- `--space-2xl: 4rem`
- `--space-3xl: 6rem`
- `--space-page-x: clamp(3.5rem, 6.8vw, 7.4rem)`
- `--space-page-top: clamp(3.25rem, 6.2vw, 6.9rem)`
- `--space-page-bottom: clamp(6rem, 10vw, 8.4rem)`

### Active tokens

```css
:root {
  --color-paper: #efe9db;
  --color-paper-2: #e7e0cf;
  --color-ink: #141210;
  --color-body: #2a2620;
  --color-card: #fbf7ee;
  --color-muted: #726b5c;
  --color-dark-body: #d8d2c2;
  --color-dark-muted: #a89f8d;
  --color-violet: #9d4edd;
  --color-blue: #3a86ff;
  --color-cyan: #1fbcd8;
  --color-green: #12b886;
  --color-yellow: #f0b429;
  --color-orange: #ff8c42;
  --color-red: #ef476f;
  --color-pink: #ff5da2;
  --font-display: "Space Grotesk", system-ui, sans-serif;
  --font-body: "Space Grotesk", system-ui, sans-serif;
  --font-mono: "JetBrains Mono", ui-monospace, monospace;
  --type-xs: clamp(0.72rem, 1.05vw, 1.2rem);
  --type-sm: clamp(0.8rem, 1.18vw, 1.32rem);
  --type-body: clamp(1.05rem, 1.55vw, 1.85rem);
  --type-lead: clamp(1.05rem, 1.72vw, 2rem);
  --type-h2: clamp(1.65rem, 3.4vw, 3.2rem);
  --type-h1: clamp(3rem, 6vw, 5.5rem);
  --type-hero: clamp(3.7rem, 7.4vw, 7rem);
  --space-xs: 0.75rem;
  --space-sm: 1rem;
  --space-md: 1.5rem;
  --space-lg: 2rem;
  --space-xl: 3rem;
  --space-2xl: 4rem;
  --space-3xl: 6rem;
  --duration-fast: 220ms;
  --duration-medium: 300ms;
  --duration-slow: 600ms;
  --ease-out: cubic-bezier(0.25, 1, 0.5, 1);
  --ease-spring: cubic-bezier(0.34, 1.56, 0.64, 1);
}
```
