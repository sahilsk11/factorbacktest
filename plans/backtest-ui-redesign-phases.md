# Backtest Page UI Redesign — Phased Implementation Plan

> Focused on the Backtest page (`/backtest`) first. Changes should be compatible with other pages (Home, Investments, Bond) via shared CSS tokens and global styles.

---

## Before You Start

- All work happens in `~/wt/ui-redesign/frontend/src/`
- Styles: CSS Modules (existing `.module.css` files) + Tailwind `globals.css` tokens
- Charts: `chart.js` via `react-chartjs-2` — already in use
- Monaco Editor: already in use — keep it, just update its theme
- Keep react-bootstrap for layout primitives (Container, Row, Col, Table) — it's working, no need to replace it yet

---

## Phase 1 — Foundation: Dark Theme Refinement & Typography

**Goal:** Establish the visual foundation — dark background, monospace numbers, cleaned-up surfaces.

### 1.1 Update `globals.css` dark palette tokens

Update the `.dark` block to shift the background darker and introduce a single accent color:

```css
.dark {
  --background: 220 14% 4%;      /* deep near-black, slightly blue-tinted */
  --foreground: 213 31% 91%;     /* warm off-white text */
  --muted: 220 14% 8%;          /* slightly elevated surface */
  --muted-foreground: 215 16% 55%;
  --accent: 38 92% 55%;          /* amber — Bloomberg-style, primary interactive color */
  --accent-foreground: 220 14% 4%;
  --border: 220 14% 14%;         /* barely-visible border */
  --input: 220 14% 14%;
  --card: 220 14% 6%;            /* card surfaces — one step above background */
  --card-foreground: 213 31% 91%;
  --primary: 38 92% 55%;         /* amber for primary buttons */
  --primary-foreground: 220 14% 4%;
  --secondary: 220 14% 12%;
  --secondary-foreground: 213 31% 91%;
  --destructive: 0 60% 45%;      /* muted red for errors */
  --destructive-foreground: 213 31% 91%;
  --ring: 38 92% 55%;            /* amber ring — visible focus indicator */
  --radius: 0.375rem;            /* slightly less rounded */
}
```

Add semantic tokens for financial data:
```css
--profit: 142 70% 45%;           /* muted green — not neon */
--loss: 0 65% 50%;               /* muted red */
--code-font: 'JetBrains Mono', 'Fira Code', 'Cascadia Mono', monospace;
```

### 1.2 Add Google Font imports

In `index.css` (or wherever fonts are loaded), add:
```css
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500&display=swap');
```

Update tailwind.config.js fontFamily to use Inter for UI and JetBrains Mono for code/numbers:
```js
fontFamily: {
  sans: ['Inter', 'system-ui', 'sans-serif'],
  mono: ['JetBrains Mono', 'Fira Code', 'Cascadia Mono', 'monospace'],
},
```

### 1.3 Clean up `App.module.css`

The `.tile` class uses a heavy box-shadow (`rgba(149, 157, 165, 0.2) 0px 8px 24px`) — replace it with a flat dark surface treatment:
```css
.tile {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: var(--radius);
  /* no shadow — depth comes from color contrast, not shadow */
}
```

Also fix `.subtext` (currently `rgb(100, 100, 100)`) to use `--muted-foreground`.

### 1.4 Monospace numbers everywhere

In every `<td>` and `<th>` that displays a number — annualized return, Sharpe ratio, factor scores, prices, allocations — add `font-mono` to ensure numbers align in columns. For react-bootstrap Table, this means either adding a CSS class or using inline styles.

Create a `globals.css` utility:
```css
.num {
  font-family: var(--code-font);
  font-variant-numeric: tabular-nums;
}
```

Apply `className="num"` to any numeric cell in tables.

### 1.5 Verify dark mode is active

The app should open in dark mode by default (no toggle needed yet). Confirm the `.dark` class is present on `<html>` or `<body>`. If the dark class is toggled, default it to always-on for now.

---

## Phase 2 — Charts: Draw-In Animation & Terminal Styling

**Goal:** Make charts feel alive and match the terminal aesthetic — muted grid lines, draw-in animation, monospace labels.

### 2.1 Chart.js global defaults

In `BacktestChart.tsx` (or in a shared `util.ts` that registers Chart.js defaults), set global defaults so every chart inherits the terminal look:

```js
import { Chart } from 'chart.js';

Chart.defaults.color = '#9ca3af';       // muted gray for labels — not black
Chart.defaults.borderColor = 'rgba(255,255,255,0.06)';  // barely-visible grid
Chart.defaults.font.family = "'JetBrains Mono', monospace";
Chart.defaults.font.size = 11;
```

### 2.2 Draw-in animation for the equity curve line

In `BacktestChart.tsx`, add animation config to the line chart options:

```js
const options: ChartOptions<"line"> = {
  animation: {
    duration: 1200,
    easing: 'easeOutQuart',
  },
  // ...
}
```

For the "line draws itself" effect, Chart.js animation is the lightweight option. If more sophisticated SVG path animation is needed later, that's phase 5.

### 2.3 Muted legend and axis styling

Update the legend and axes in `BacktestChart.tsx`:

```js
legend: {
  position: 'top',
  labels: {
    color: "#9ca3af",    // muted — not bright white
    font: { family: "'JetBrains Mono'", size: 11 },
    boxWidth: 12,
    padding: 16,
  }
},
scales: {
  x: {
    grid: { color: 'rgba(255,255,255,0.04)' },  // very subtle grid
    ticks: { color: '#6b7280', font: { family: "'JetBrains Mono'", size: 10 } },
  },
  y: {
    grid: { color: 'rgba(255,255,255,0.04)' },
    ticks: {
      color: '#6b7280',
      font: { family: "'JetBrains Mono'", size: 10 },
      callback: (val) => val.toFixed(1) + '%'
    },
  }
}
```

### 2.4 Color scheme for datasets

The current chart uses Chart.js's automatic color generation (which produces bright, varied colors). Replace with a controlled palette:

```js
const SERIES_COLORS = {
  primary: '#f59e0b',    // amber — the main factor line
  benchmark: '#4b5563',  // muted gray — benchmarks recede visually
  grid: 'rgba(255,255,255,0.04)',
};
```

Apply `primary` to the first factor series, `benchmark` to all benchmark series, and additional muted colors for subsequent factor series.

### 2.5 Update `FactorSnapshot.tsx` doughnut chart

Apply the same muted color palette and font treatment to the `AssetBreakdown` doughnut chart. The doughnut should use muted colors from the same palette (not the default Chart.js palette).

---

## Phase 3 — Keyboard Shortcuts & Command Palette

**Goal:** Make the backtest page keyboard-navigable. Introduce a global keyboard shortcut system.

### 3.1 Keyboard shortcut hook

Create a new file `frontend/src/@/hooks/useKeyboardShortcuts.ts`:

```ts
import { useEffect } from 'react';

type ShortcutHandler = (e: KeyboardEvent) => void;
type ShortcutMap = Record<string, ShortcutHandler>;

const globalHandlers: ShortcutMap = {};

export function useGlobalKeyboardShortcut(key: string, handler: ShortcutHandler) {
  useEffect(() => {
    globalHandlers[key] = handler;
    return () => { delete globalHandlers[key]; };
  }, [handler]);
}

export function GlobalKeyboardListener() {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      // Don't fire when typing in inputs or Monaco editor
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || e.target?.['monoEditor']) return;
      const h = globalHandlers[`${e.metaKey || e.ctrlKey ? 'mod+' : ''}${e.key}`];
      h?.(e);
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);
}
```

Add `GlobalKeyboardListener` to `App.tsx` so it's always active.

### 3.2 Shortcuts for the Backtest page

Add these shortcuts to `Backtest.tsx` via `useGlobalKeyboardShortcut`:

| Shortcut | Action |
|---|---|
| `mod+r` | Run backtest (triggers the existing run button) |
| `mod+s` | Save / bookmark strategy |
| `mod+\` | Focus the Monaco editor (factor expression input) |
| `Escape` | Clear current backtest results (same as Clear button) |
| `?` | Show shortcut reference overlay |

### 3.3 Shortcut reference overlay

Create `frontend/src/@/components/ShortcutOverlay.tsx` — a small modal that lists all available shortcuts. Triggered by `?`. Styled in dark theme with monospace shortcut keys.

### 3.4 Focus management

Ensure tab order through the page is logical:
- Factor expression editor → Backtest Start/End → Num Assets → Run button → Chart area
- Every `onClick` handler should also be reachable and activatable via keyboard (buttons are already focusable — this is mainly for any divs used as click targets)

---

## Phase 4 — Layout: Multi-Panel Structure

**Goal:** Restructure the Backtest page layout from a simple two-column flex into a more flexible, dense, terminal-style panel arrangement.

### 4.1 Current layout analysis

Currently:
```
[ Navbar ]
[ FactorForm + BenchmarkSelector (flex 2) ] [ Chart + Inspector (flex 4) ]
```

### 4.2 New layout concept

The page is divided into three resizable regions:

```
[ Navbar ]
[  Top Bar: strategy name | dates | run status                              ]
[ Left Panel (320px min) ] [ Center Panel (chart, flex 1) ] [ Right Inspector Panel (280px min) ]
```

- **Top Bar:** Single row showing strategy name, date range, run button, clear button. Dense, monospace numbers.
- **Left Panel:** Form inputs — factor expression (Monaco), parameters, benchmarks. Collapsible to icon-only sidebar.
- **Center Panel:** Equity curve chart. The dominant visual element. Fills remaining space.
- **Right Inspector Panel:** Slide-in. Shows holdings table, doughnut allocation chart, and metrics. Toggled by `I` key or the chart click.

### 4.3 Implementation approach

Use CSS Grid for the main layout (not flex), with named areas:
```css
.backtest-layout {
  display: grid;
  grid-template-columns: 320px 1fr 280px;
  grid-template-rows: 48px 1fr;
  grid-template-areas:
    "topbar topbar topbar"
    "left center right";
  height: calc(100vh - 56px); /* minus navbar */
  gap: 1px;
  background: hsl(var(--border)); /* gap color */
}
```

Panels fill their grid areas with `overflow: hidden` or `overflow: auto` as appropriate.

### 4.4 Panel collapse behavior

- Left panel can collapse to a 48px icon strip (hamburger to expand). Collapsed state persisted in localStorage.
- Right inspector panel is hidden by default until a backtest is run. `I` key toggles it.
- All collapse/expand transitions: 200ms ease-out slide.

---

## Phase 5 — Animation & Polish

**Goal:** Add motion that communicates state, not just decoration.

### 5.1 Page/panel transitions

- When the inspector panel slides in from the right, use a CSS transition (`transform: translateX(100%)` → `translateX(0)`, 200ms ease-out).
- When backtest results load (new data arrives in the chart), the line should animate in (Chart.js animation, already configured in Phase 2).

### 5.2 Loading skeletons

While a backtest is running, show skeleton loaders in the chart area and inspector — pulsing dark rectangles (not spinners). Use a CSS animation:

```css
@keyframes skeleton-pulse {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 0.8; }
}
.skeleton {
  background: hsl(var(--muted));
  animation: skeleton-pulse 1.5s ease-in-out infinite;
}
```

### 5.3 Number transitions

When metrics update (annualized return, Sharpe, etc.), animate the number counting from old to new value. A lightweight approach: for each numeric display, on value change animate `innerText` from old to new over 400ms.

### 5.4 Hover and focus micro-interactions

- Table rows: subtle background shift on hover (`background: hsl(var(--muted))`), not color change.
- Buttons: `transition: opacity 0.15s` on hover. Focus ring is already amber from Phase 1.
- Monaco editor: already has its own focus style — just ensure the focus ring color matches the amber accent.

### 5.5 SVG path draw-in (optional, if Chart.js animation feels insufficient)

If the Chart.js animation isn't smooth enough for the "line draws from origin" effect, implement a custom SVG line animation using `stroke-dasharray` / `stroke-dashoffset` on a `<path>` element inside a `<svg>`. This gives full control over the draw speed and easing. Libraries like `framer-motion` or `vivus` can help, but a plain CSS implementation is ~20 lines.

---

## Phase 6 — Component Style Refinement

**Goal:** Refine individual components to match the terminal aesthetic.

### 6.1 Form inputs (non-Monaco)

For text inputs, selects, and number inputs in `Form.tsx` — apply the dark surface treatment:
```css
/* In Form.module.css or globals.css */
input[type="text"],
input[type="number"],
select {
  background: hsl(var(--input));
  border: 1px solid hsl(var(--border));
  color: hsl(var(--foreground));
  border-radius: var(--radius);
  padding: 6px 10px;
  font-family: inherit;
  font-size: 13px;
}
input[type="number"] {
  font-family: var(--code-font);  /* numbers in monospace */
}
```

### 6.2 Buttons

Refine `formStyles.backtest_btn` and any other buttons:
- Primary (Run Backtest): amber background, dark text. Solid.
- Secondary (Clear, Cancel): transparent background, amber border, amber text. Ghost/outline style.
- All buttons: 200ms transition on hover. No box-shadow.

### 6.3 Navigation bar

The `Navbar` from react-bootstrap uses Bootstrap's dark theme. Refine it:
- Remove Bootstrap's default navbar background — use `background: hsl(var(--card))` instead.
- Border-bottom: 1px solid `hsl(var(--border))`.
- Nav links: muted by default, amber on hover/active.
- Brand: `factor.trade` in Inter, 14px, medium weight, amber on hover.

### 6.4 Tables (holdings, metrics, allocations)

- `th`: muted-foreground color, uppercase, 11px, letter-spacing 0.05em. Bottom border `1px solid hsl(var(--border))`.
- `td`: foreground color, `font-mono`, `tabular-nums` for alignment.
- `tr`: no border. Hover: `background: hsl(var(--muted))`.
- Alternating row backgrounds: off by default (too much visual noise at data-dense scales).

### 6.5 Tabs (in Inspector panel)

Replace Bootstrap `Nav variant="tabs"` with minimal custom tabs:
- Active tab: amber bottom border (2px), amber text.
- Inactive tab: muted text, no background.
- No background color on the tab bar itself — just a bottom border.

---

## Phase 7 — Command Palette (Cmd+K)

**Goal:** A global command palette that provides keyboard access to all app actions.

### 7.1 Command palette UI

Create `frontend/src/@/components/CommandPalette.tsx`:
- Full-screen overlay with a centered modal box (400px wide, dark surface).
- Text input at the top for fuzzy search.
- Scrollable list of commands below.
- Each command shows: name, category group, keyboard shortcut (right-aligned in muted text).
- `Escape` or clicking outside closes it.
- `Enter` on a command executes it.
- Arrow keys navigate the list.

### 7.2 Command registry

Create `frontend/src/@/lib/commands.ts`:
```ts
type Command = {
  id: string;
  label: string;
  category: string;
  shortcut?: string;
  handler: () => void;
};

export const commands: Command[] = [
  { id: 'nav-home', label: 'Go to Home', category: 'Navigation', shortcut: 'g h', handler: () => navigate('/') },
  { id: 'nav-backtest', label: 'Go to Backtest', category: 'Navigation', shortcut: 'g b', handler: () => navigate('/backtest') },
  { id: 'run-backtest', label: 'Run Backtest', category: 'Actions', shortcut: 'mod+r', handler: () => runBacktest() },
  { id: 'save-strategy', label: 'Save Strategy', category: 'Actions', shortcut: 'mod+s', handler: () => saveStrategy() },
  { id: 'clear-results', label: 'Clear Results', category: 'Actions', handler: () => clearResults() },
  { id: 'toggle-inspector', label: 'Toggle Inspector Panel', category: 'View', shortcut: 'i', handler: () => toggleInspector() },
  { id: 'show-shortcuts', label: 'Show Keyboard Shortcuts', category: 'Help', shortcut: '?', handler: () => showShortcutOverlay() },
  // ... more commands
];
```

### 7.3 Fuzzy search

Implement lightweight fuzzy matching on command labels. No library needed for <100 commands — a simple `label.toLowerCase().includes(query.toLowerCase())` filter is sufficient. Optionally score by match position.

### 7.4 Trigger and integration

- `Cmd+K` (Mac) / `Ctrl+K` (Windows) opens the palette — add to the global keyboard listener from Phase 3.
- When open, the palette traps focus. `Escape` closes and returns focus to the previously focused element.
- The command palette is used for navigation AND action execution.

---

## Phase Dependencies

```
Phase 1 (Foundation)
  └─ Phase 2 (Charts)       ← charts pick up new colors/fonts
  └─ Phase 3 (Shortcuts)     ← independent, can go anytime
  └─ Phase 4 (Layout)       ← independent, can go anytime
  └─ Phase 6 (Components)   ← components pick up new CSS vars
Phase 4 (Layout)
  └─ Phase 5 (Animation)    ← animation on layout transitions
Phase 5 (Animation)
  └─ Phase 7 (Cmd Palette)  ← palette is just a modal, independent
```

**Suggested execution order:** 1 → 3 → 2 → 4 → 6 → 5 → 7

---

## Files to Modify

| File | Phases |
|---|---|
| `frontend/src/globals.css` | 1, 6 |
| `frontend/src/index.css` | 1 |
| `frontend/tailwind.config.js` | 1 |
| `frontend/src/App.module.css` | 1, 6 |
| `frontend/src/App.tsx` | 3, 7 |
| `frontend/src/@/hooks/useKeyboardShortcuts.ts` | 3 |
| `frontend/src/@/components/ShortcutOverlay.tsx` | 3 |
| `frontend/src/@/components/CommandPalette.tsx` | 7 |
| `frontend/src/@/lib/commands.ts` | 7 |
| `frontend/src/pages/Backtest/Backtest.tsx` | 3, 4, 5 |
| `frontend/src/pages/Backtest/Backtest.module.css` | 4, 5 |
| `frontend/src/pages/Backtest/BacktestChart.tsx` | 2, 5 |
| `frontend/src/pages/Backtest/Form.tsx` | 4, 6 |
| `frontend/src/pages/Backtest/Form.module.css` | 6 |
| `frontend/src/pages/Backtest/FactorSnapshot.tsx` | 2, 5, 6 |
| `frontend/src/pages/Backtest/FactorSnapshot.module.css` | 6 |
| `frontend/src/common/Nav.tsx` | 6 |
| `frontend/src/common/Nav.module.css` | 6 |
