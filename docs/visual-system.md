# GoTreeSitter documentation design

## Visual System

### Territory

Use a playful **Paper & Ink** territory with neo-brutalist technical
accents. Warm paper surfaces carry dense technical content. Hard ink borders,
square corners, bright functional labels, and offset shadows expose structure.
Do not add gradients, soft shadows, decorative glass, or large motion.

### Typography

Use a compact minor-third scale with a 1.2 ratio.

- Display: Space Grotesk 700.
- Body: Space Grotesk 400, 500, and 600.
- Metadata and controls: JetBrains Mono 400, 500, and 700.

Use these responsive steps:

- Extra small: `clamp(0.6875rem, 0.67rem + 0.08vw, 0.75rem)`.
- Small: `clamp(0.8125rem, 0.78rem + 0.12vw, 0.875rem)`.
- Base: `clamp(1rem, 0.96rem + 0.18vw, 1.0625rem)`.
- Large: `clamp(1.2rem, 1.12rem + 0.35vw, 1.35rem)`.
- Extra large: `clamp(1.44rem, 1.3rem + 0.6vw, 1.75rem)`.
- Two extra large: `clamp(1.728rem, 1.5rem + 1vw, 2.25rem)`.
- Three extra large: `clamp(2.074rem, 1.7rem + 1.7vw, 3rem)`.
- Four extra large: `clamp(2.488rem, 2rem + 2.5vw, 3.75rem)`.

### Color architecture

Use warm paper for approximately 60 percent of each view. Use cards and
paper-two surfaces for approximately 30 percent. Use the existing bright
functional accents for the remaining 10 percent.

- Dominant paper: `#efe9db`.
- Secondary card: `#fbf7ee`.
- Secondary paper: `#e7e0cf`.
- Primary ink: `#141210`.
- Readable muted ink: `#655e51`.
- Legacy decorative muted ink: `#726b5c`.
- Violet: `#9d4edd`.
- Blue: `#3a86ff`.
- Cyan: `#1fbcd8`.
- Green: `#12b886`.
- Yellow: `#f0b429`.
- Orange: `#ff8c42`.
- Red: `#ef476f`.
- Pink: `#ff5da2`.

Primary ink has a 15.44:1 contrast ratio on paper and 17.48:1 on cards.
It meets Web Content Accessibility Guidelines (WCAG) AAA. Readable muted ink
has a 5.30:1 ratio on paper and 6.00:1 on cards. It meets WCAG AA. Use ink,
not an accent color, for text on bright accents. Blue links use bold text and
visible underlines where surrounding prose needs more than color.

### Motion

Use minimal motion. Apply only focus, hover, and pressed-state transitions.
Use 150 milliseconds for immediate feedback and 200 milliseconds for a gentle
settle. Use `cubic-bezier(0.16, 1, 0.3, 1)` for the standard ease-out curve.
Reserve `cubic-bezier(0.34, 1.56, 0.64, 1)` for small pressed-state feedback.
Disable nonessential transitions when the user requests reduced motion.
Use native `details` expansion without animation.

### Spacing

Use a compact 4-pixel base:

- Extra small: `0.25rem`.
- Small: `0.5rem`.
- Medium: `0.75rem`.
- Large: `1rem`.
- Extra large: `clamp(1.25rem, 1rem + 1vw, 1.5rem)`.
- Two extra large: `clamp(1.5rem, 1.1rem + 1.8vw, 2rem)`.
- Three extra large: `clamp(2rem, 1.5rem + 2.4vw, 3rem)`.

### Token contract

New components must use this token block. Existing tokens remain as aliases
for the established site.

```css
:root {
  --font-display: 'Space Grotesk', system-ui, sans-serif;
  --font-body: 'Space Grotesk', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', ui-monospace, monospace;

  --text-xs: clamp(0.6875rem, 0.67rem + 0.08vw, 0.75rem);
  --text-sm: clamp(0.8125rem, 0.78rem + 0.12vw, 0.875rem);
  --text-base: clamp(1rem, 0.96rem + 0.18vw, 1.0625rem);
  --text-lg: clamp(1.2rem, 1.12rem + 0.35vw, 1.35rem);
  --text-xl: clamp(1.44rem, 1.3rem + 0.6vw, 1.75rem);
  --text-2xl: clamp(1.728rem, 1.5rem + 1vw, 2.25rem);
  --text-3xl: clamp(2.074rem, 1.7rem + 1.7vw, 3rem);
  --text-4xl: clamp(2.488rem, 2rem + 2.5vw, 3.75rem);

  --color-paper: #efe9db;
  --color-paper-2: #e7e0cf;
  --color-card: #fbf7ee;
  --color-ink: #141210;
  --color-text-muted: #655e51;
  --color-violet: #9d4edd;
  --color-blue: #3a86ff;
  --color-cyan: #1fbcd8;
  --color-green: #12b886;
  --color-yellow: #f0b429;
  --color-orange: #ff8c42;
  --color-red: #ef476f;
  --color-pink: #ff5da2;

  --space-xs: 0.25rem;
  --space-sm: 0.5rem;
  --space-md: 0.75rem;
  --space-lg: 1rem;
  --space-xl: clamp(1.25rem, 1rem + 1vw, 1.5rem);
  --space-2xl: clamp(1.5rem, 1.1rem + 1.8vw, 2rem);
  --space-3xl: clamp(2rem, 1.5rem + 2.4vw, 3rem);

  --duration-fast: 150ms;
  --duration-base: 200ms;
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1);
  --ease-spring: cubic-bezier(0.34, 1.56, 0.64, 1);
}
```
