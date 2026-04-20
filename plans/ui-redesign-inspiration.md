# Factor UI Redesign — High-Level Inspiration Plan

> **Goal:** Transform the Factor UI into a keyboard-driven, data-dense trading terminal aesthetic — dark, precise, and fast. Think Bloomberg Terminal meets Linear.app meets a well-designed code editor.

---

## 1. Visual Identity — "The Terminal Aesthetic"

**Reference points:** Bloomberg Terminal, Linear.app, Vercel Dashboard, dark.design

### Color Palette
The current palette is a generic blue-gray dark theme. We should move toward something more purposeful:

- **Background:** Near-black (#0E0E10 or similar — deep, not pure black, reduces eye strain). The current `224 71% 4%` is in the right ballpark but can go darker and more neutral.
- **Surface / Cards:** One step off-background — elevated dark surfaces with subtle borders, no heavy shadows. Think Linear's layered dark surfaces.
- **Text:** High-contrast off-white for primary text (not pure white — it's too harsh for long sessions). Warm whites or cool grays depending on section.
- **Accent:** A single strong accent color for interactive states — not blue everywhere. Bloomberg uses amber. Linear uses a per-workspace custom color. Consider a muted teal or amber for Factor.
- **Success / Danger:** Keep green/red for P&L but make them muted by default — glowing neon greens and reds are loud in a dense UI. Desaturate slightly.
- **Border / Dividers:** Subtle, low-contrast — barely visible lines that define structure without visual noise. The current `--border: 216 34% 17%` is a reasonable starting point but should be refined.

### Typography
- **UI Text:** A clean sans-serif for labels, navigation, buttons. Inter or Geist Sans are common. The current app uses system fonts — moving to a intentional choice here elevates the feel significantly.
- **Code / Factor Expressions:** JetBrains Mono, Fira Code, or Cascadia Mono — ligature-enabled monospace that matches the Monaco Editor already in use for FactorExpressionInput. Consistent font stack across code and non-code elements.
- **Numeric / Data:** Monospace for all numbers, prices, percentages. This is non-negotiable in a trading terminal — columns that don't align because of proportional fonts look unprofessional. Every number should be in a fixed-width font.

### Spatial System
- **Dense, not cramped:** The goal is information richness without clutter. Tight padding (8px-16px in UI elements), but with deliberate whitespace between logical sections. Vercel's dashboard does this well.
- **No decorative chrome:** No gradients on backgrounds, no unnecessary borders, no drop shadows that create depth. Flat surfaces, defined by color contrast alone.
- **Thin borders, not thick:** 1px borders where needed. The current `radius: 0.5rem` gives slightly rounded corners — that's fine, but the focus should be on the content, not the containers.

---

## 2. Charts & Data Visualization — "Animated Build-Out"

**Reference points:** TradingView, Vercel Analytics, subframe.com interactive chart examples

### Chart Animation Philosophy
Charts should feel *alive*, not static. When data loads or changes, the transition should communicate "this is a real-time, changing system":

- **Draw-in animation:** When a chart first renders or new data loads, lines/bars should animate from origin to final position — not just appear. TradingView's charts do this on ticker switch. For Factor, this could apply to equity curves, factor returns, and performance charts.
- **Smooth transitions:** Cross-fading between datasets, smooth interpolation when date ranges change. No jarring jumps.
- **Progressive reveal:** For long time series, consider revealing data progressively as the user scrolls or navigates — like a path being drawn in real time. SVG stroke-dasharray animation is the underlying technique (see Vivus.js for implementation reference).

### Chart Style
- **Dark, muted grid lines:** Grid lines should be subtle — low-contrast against the background, not bright white. The data itself should be the loudest element.
- **Single accent for key series:** If showing multiple series (factor vs benchmark), use one prominent color for the primary and muted gray for the benchmark. Not a rainbow.
- **Tooltips:** Minimal, keyboard-navigable tooltips — not hover-dependent. The user should be able to step through data points with arrow keys.

### Chart Types to Consider
- **Candlestick / OHLC:** For bond data or granular price history.
- **Area charts:** For cumulative return series — the filled area gives a sense of magnitude.
- **Bar charts:** For factor returns at each rebalance date, especially with the draw-in animation.
- **Correlation matrices or heatmaps:** If showing factor correlations across time, a heatmap with subtle color gradients (not rainbow) communicates correlation strength.

---

## 3. Keyboard-First Interaction — "The Power User Layer"

**Reference points:** Linear.app, Raycast, Superhuman, Vim

### Command Palette (Cmd+K)
The single highest-impact UX improvement. A global command palette — triggered by Cmd+K (or Ctrl+K) — that gives keyboard access to everything:

- Navigate between pages (Backtest, Bond, Home, Investments)
- Run a backtest
- Save / load strategies
- Switch between light/dark mode
- Toggle panels (show/hide the inspector, chart, form)
- Trigger exports

The palette fuzzy-searches commands as you type, shows keyboard shortcuts inline, and groups recent commands at the top. Raycast is the canonical reference; Linear's implementation in a web app is also excellent.

### Keyboard Navigation Throughout
- **Tab navigation:** Full logical tab order through every interactive element. No keyboard traps.
- **Arrow keys for lists:** Strategies, holdings, symbol lists — all navigable with arrow keys, not just click.
- **Escape to close:** Every modal, dropdown, tooltip closes with Escape. No exceptions.
- **Shortcuts for common actions:** `R` to run backtest, `S` to save strategy, `?` to show the shortcut reference overlay.
- **Focus indicators:** Visible, high-contrast focus rings on every interactive element when navigating by keyboard. The current ring color (`--ring: 216 34% 17%`) may be too subtle — consider making it brighter.

### Contextual Panels
- **Sliding inspector panel:** Currently FactorSnapshot.tsx shows factor details. This should be a slide-in panel triggered by a keyboard shortcut (e.g., `I` for inspect), not always visible占用 screen real estate.
- **Collapsible sidebar:** Navigation that doesn't dominate the screen. Collapse to icons only; expand on hover or with a keyboard shortcut.

---

## 4. Layout & Structure — "Dense but breathable"

**Reference points:** Bloomberg Terminal, Vercel Dashboard, TradingView

### Information Density
Factor is a financial analysis tool — users expect density. The redesign should lean into this rather than fighting it:

- **Multi-panel layout:** The Backtest page already has form + chart + inspector. The redesign should formalize this as a resizable multi-panel workspace, similar to a code editor or trading terminal. Panels snap to grid, and users can rearrange them.
- **Fixed chrome, scrolling content:** Navigation and primary controls stay fixed; data areas scroll independently. This keeps context visible while allowing large datasets.
- **Tabs for context-switching:** If multiple strategies are open, tabs at the top (keyboard-navigable) rather than routing between pages. Think VS Code tabs, not browser tabs.

### Responsive Strategy
- **Desktop-first:** This UI is optimized for a large monitor and a keyboard. Mobile is secondary — but should still be usable for viewing results, not authoring.
- **Command palette as primary navigation on mobile:** Since there's no hover on mobile, the command palette becomes the main navigation interface.

---

## 5. Animation & Micro-interactions — "Motion with purpose"

**Reference points:** Apple Keynote animations, Framer motion, Linear's UI transitions

### Principles
- **Subtle, not flashy:** Animations should communicate state changes, not entertain. Every animation has a reason.
- **Fast:** 150-300ms for micro-interactions. 300-500ms for panel transitions. Nothing longer.
- **Easing, not linear:** Use ease-out for elements entering, ease-in for elements leaving. Linear motion feels robotic.

### Specific Animations
- **Page transitions:** Cross-fade between views, not a hard cut.
- **Panel slide-ins:** Inspector panel slides in from right. Form panel slides in from left. Chart area expands to fill space.
- **Data loading states:** Skeleton loaders (pulsing dark rectangles) while data fetches — not spinners. Spinners are 1990s.
- **Number transitions:** When P&L or returns update, numbers animate to the new value rather than snapping. A counting-up effect for final values.
- **Chart draw-in:** SVG path animation — stroke-dashoffset transitions from full offset to 0 as data loads.

---

## 6. Component Style — "Refined, not reinvented"

### Buttons
- Minimal button styles — no heavy gradients or shadows. Outline or ghost buttons for secondary actions. Solid buttons for primary actions (run backtest, save).
- Consistent border-radius across all buttons (current `0.5rem` is fine).
- Keyboard shortcut hints shown on buttons where applicable.

### Inputs
- Monaco Editor already handles factor expression input — keep it, but ensure its theme matches the new UI palette exactly.
- For simple text/number inputs: dark surface background, subtle border, monospace for numbers.
- No input backgrounds that are pure black — use the elevated surface color so inputs are distinguishable from the page background.

### Tables
- Every table uses monospace for numbers.
- Row hover states are subtle — background shifts, not color changes.
- Sortable columns show direction indicators and are keyboard-accessible.
- Alternating row backgrounds are optional — if spacing is tight, no zebra striping; if space allows, very subtle alternating backgrounds.

### Modals / Dialogs
- Centered, with a dimmed backdrop. Escape closes.
- No heavy drop shadows — a subtle border and slightly elevated background is enough.
- The command palette itself *is* the primary modal — for everything else, keep modals small and infrequent.

---

## 7. Color System Implementation Notes

### Tokens to Define
The current CSS variable system in `globals.css` is the right foundation. Key tokens to update for the new aesthetic:

| Token | Current Value | Target Direction |
|---|---|---|
| `--background` | `224 71% 4%` | Go darker: `220 14% 4%` or similar |
| `--foreground` | `213 31% 91%` | Warm off-white, not cool gray |
| `--border` | `216 34% 17%` | Even subtler: `220 14% 12%` |
| `--accent` | (inherited) | A single strong color — amber or teal |
| `--ring` | `216 34% 17%` | Brighter for keyboard focus visibility |
| `--muted` | `223 47% 11%` | Slightly elevated from background |

### Semantic Colors
Define semantic tokens on top of the base palette:
- `--profit: <green>` — muted green for gains
- `--loss: <red>` — muted red for losses
- `--accent: <teal-or-amber>` — primary interactive color
- `--code: <monospace-blue>` — syntax highlighting in Monaco

---

## 8. Inspiration Sources Summary

| Reference | What to Steal |
|---|---|
| **TradingView** | Draw-in chart animations, real-time data feel, dense information layout |
| **Linear.app** | Keyboard-first navigation, command palette, minimal chrome, dark surface layering |
| **Raycast** | Command palette UX, fuzzy search, shortcut hints inline |
| **Bloomberg Terminal** | Information density, monospace numbers, amber accent on dark background |
| **Vercel Dashboard** | Clean dark surfaces, subtle borders, precise typography, no decorative noise |
| **Vivus.js / Framer Motion** | SVG path draw-in animation technique for charts |
| **JetBrains Mono / Fira Code** | Monospace font with ligatures for code + numeric data |
| **dark.design aggregator** | General dark UI inspiration across domains |

---

## 9. Out of Scope for This Plan

- Any specific component library adoption or abandonment (shadcn/ui is fine to keep)
- Implementation of the backend API
- Changes to the data models or database schema
- Specific color hex values (those are design-time decisions)
- Specific animation library choices (Framer Motion vs CSS transitions vs React Spring)
- Mobile-first redesign — desktop is the primary use case
