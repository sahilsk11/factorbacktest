# frontend-v2 — North Star

> **Goal:** Replace the Create-React-App `frontend/` with a modern, AI-built React app that *looks like* TradingView and Robinhood, *feels like* Linear and Vercel, and *behaves like* a serious financial tool. Rip-and-replace, page-by-page, behind a path-based switch — no big-bang cutover.

This doc captures the long-running direction so future AI sessions (and future-me) don't have to re-derive it from scratch. It is intentionally **not** a PR-by-PR sequence — it's a list of everything we eventually want, from which each PR will pick a slice.

It supersedes the earlier think-piece in `plans/ui-redesign-inspiration.md` (which leaned Bloomberg-terminal). The direction has since converged on something more consumer-fintech (Robinhood/Vercel) than power-user-terminal.

---

## 1. Why rewrite, not refactor

The legacy `frontend/` is ~6,100 lines of TypeScript that has grown some real cruft:

- `frontend/src/pages/Backtest/Form.tsx` is **1,028 lines** and takes ~25 props.
- `frontend/src/pages/Backtest/Backtest.tsx` (375 lines) has self-flagged hacks (`// super hacky until we can refactor`, render-phase `navigate()`).
- `frontend/src/pages/Bond/Bond.tsx` is **605 lines** in a single file.
- The stack is Create React App (deprecated 2023), Bootstrap 5 + react-bootstrap **mixed with** Tailwind + shadcn/ui — three competing styling systems.
- Errors are surfaced via `alert()`, raw `fetch` is everywhere, and there's a custom legacy `userID` cookie alongside the newer cookie session.

**Why a rewrite is the right call here, even though it usually isn't:**

- The frontend is a thin shell over ~15 Go endpoints. There is no business logic in the React code that we'd lose — all of it lives in `internal/` on the backend.
- The Go API is the contract. As long as we re-implement against the same endpoints, behavioral parity is mechanical.
- The legacy code's most-broken patterns (god components, prop drilling, alert-driven errors) are systemic. Refactoring in place would touch every file anyway.

**The real risks (not "this isn't best practice"):**

- Scope creep — "while we're at it, let's also fix X" turns 4 weekends into 4 months.
- Silent regressions — easy to drop bookmarks, the GPT-assisted factor input, URL-shareable backtests, etc., if we don't enumerate them.

**Mitigations that we agreed on:**

- Both apps run side-by-side. Old app stays at `/*`, new app lives at `/v2/*` until each page is approved, then routes flip.
- A feature inventory (`PARITY.md` — to be added) enumerates every legacy capability marked **port / port-and-improve / drop / defer**. This is the kill switch against silent regressions.
- One PR per page. You play with each before approving the next.

**One thing worth preserving verbatim:** `frontend/src/common/BacktestLoading/` — the SSE streaming hook + parser. The event contract is real and the code is good. Port 1:1.

---

## 2. North Star aesthetic

> **"Calm canvas, confident data."**

The unifying aesthetic across our reference set (TradingView, Robinhood, Coinbase, Stripe, Linear, Vercel) is dark-first surfaces, data as the loudest element, and motion that feels alive without being playful.

### Concrete patterns we're stealing

- **Dark-first**: near-black page (`#0A0A0B`), cards one shade lighter (`#111214`), elevated surfaces (`#1A1B1F`). 1px hairline borders at ~6–10% white opacity.
- **Gradient area fills under lines**: vertical gradient from line color at ~25–35% opacity at top to 0% at baseline. Single most identifiable Robinhood / Coinbase / Vercel move.
- **Smooth-but-honest curves**: `d3.curveMonotoneX` for equity curves, straight segments for tick-precise data. Never `curveBasis` — it lies.
- **Sparse axes**: 3–5 y-axis labels max, no axis line, gridlines either gone or one dotted horizontal at 5% opacity.
- **Floating right-edge price label**: small pill at the latest value, color-matched to gain/loss (TradingView signature). On Robinhood the entire line color flips green/red vs. session open.
- **Crosshair on hover**: vertical dotted line + circular puck on the curve + floating tooltip card with date and value, in tabular-mono digits.
- **Restrained motion**: `cubic-bezier(0.4, 0, 0.2, 1)` or Framer's `easeOut`, durations 150–600ms. Never bouncy.

### Animation patterns

- **Draw-on**: line draws left-to-right over 600–900ms on mount; gradient fill fades in slightly behind the stroke.
- **Tween between data updates**: when the timeframe changes (1D → 1W), each y-value lerps from old to new over ~400ms. TradingView Lightweight Charts handles this natively.
- **Hover transitions**: crosshair fades in over 80–120ms; tooltip slides 4–8px and fades.
- **Number readouts**: rolling-digit / `requestAnimationFrame` tween (~150ms) so values feel alive when scrubbing.
- **Pulsing "live" dot**: glowing circle at the latest point with a 1.2s expanding ring (Robinhood "laser mode").
- **Zoom/pan**: inertia + rubber-banding at edges; pinch and wheel zoom rescale Y to fit the visible window.

### Color palette (dark mode default)

| Token | Hex | Usage |
|---|---|---|
| `bg-page` | `#0A0A0B` | Page background |
| `bg-card` | `#111214` | Card/panel background |
| `bg-elevated` | `#1A1B1F` | Modals, popovers, hover states |
| `border-subtle` | `rgba(255,255,255,0.06)` | Hairline borders |
| `border-default` | `rgba(255,255,255,0.10)` | Hover/active borders |
| `text-primary` | `#FAFAFA` | Headlines, KPIs |
| `text-secondary` | `#A1A1AA` | Body, labels |
| `text-tertiary` | `#71717A` | Captions, axis labels |
| `gain` | `#10B981` | Positive deltas, gain lines (emerald-500) |
| `loss` | `#EF4444` | Negative deltas, loss lines (red-500) |
| `accent` | `#3B82F6` or `#7C3AED` | Benchmark lines, primary CTAs |
| `warn` | `#F59E0B` | Drawdown bands, soft warnings |

Gain/loss gradient stops use the same hue at 25% opacity → 0%.

### Typography

- **UI / headings**: Geist Sans (Inter as fallback). `font-feature-settings: "ss01", "cv11"` for cleaner digits.
- **Numbers, tables, tooltips, axis labels**: Geist Mono or JetBrains Mono with `font-variant-numeric: tabular-nums slashed-zero`. **This single CSS rule is the difference between "fintech" and "homework project."**
- **Code (factor expressions in Monaco)**: matches the table mono — JetBrains Mono.
- **Sizes**: 12px tabular for axis labels, 14px body, 24–32px for headline KPIs, 48px+ only for the hero portfolio value.

---

## 3. Stack

| Concern | Choice | Rationale |
|---|---|---|
| Build tool | **Vite** | Static `dist/` output served by the existing Go binary; no Node runtime in production; smallest mental model. Picked over Next.js because we have no SSR/SEO/edge needs and we deploy to Fly, not Vercel. |
| UI framework | **React 19 + TypeScript** strict | Continuity with what's there; modern React is fine. |
| Styling | **Tailwind v4** + design tokens defined once in the Tailwind config | Same primitives the legacy app already uses for new code. |
| Component primitives | **shadcn/ui** + **Tremor** for fintech-specific blocks (KPI cards, sparkline-in-table, donuts) | shadcn for the chrome, Tremor for the parts shaped like our app. |
| Charting | **TradingView Lightweight Charts** for the equity curve, drawdown chart, benchmark overlay; **Recharts** (or a Visx one-off) for non-time-series like factor exposure | Apache-2.0, ~45KB, purpose-built financial UX. WebGL is unnecessary at backtesting scale. |
| Animation | **Framer Motion** | Page transitions, KPI number rolls, tooltip enter/exit. Durations 150–400ms. |
| Data layer | **TanStack Query** + a typed `apiClient` wrapper around `fetch` | Replaces every raw `fetch` + `alert(err)` with real loading/error/cache. |
| Tables | **TanStack Table** | Sticky headers, virtualization, tabular nums for holdings/trade-log. |
| Routing | **React Router v7** | Plain client-side routing; no framework overhead. |
| Theme | **Custom hook + `localStorage`** | Dark default, light optional. No `next-themes` since we're not on Next. |
| State (ephemeral, cross-component) | **Zustand** *if needed* | For things like selected timeframe, hovered date shared between chart and KPIs. Don't reach for it until prop-drilling actually hurts. |
| Tests | **Vitest** for unit, **Playwright** for E2E | Add when there's something worth testing. Not in scaffolding. |

### Stack decisions we explicitly rejected

- **Next.js**: gives us SSR/RSC/`next/image`/API routes, none of which we use. Adds a Node runtime and a second mental model (`'use client'`, RSC payload, caching layers) that the project doesn't need. The "looks like Vercel" aesthetic is **Tailwind + shadcn + Geist + Framer Motion + Tremor**, not Next.js. Vite gets us the same look with less complexity.
- **ECharts, Highcharts, Nivo, raw D3**: research scored these worse than TradingView Lightweight Charts for our specific shape (financial time series).
- **Storybook, Husky, lint-staged, conventional-commits, Sentry**: not worth the friction at this size. Reconsider if/when there are multiple human contributors.
- **`exactOptionalPropertyTypes`, `noUncheckedIndexedAccess`**: real safety nets but generate noise that AI tends to silence with `as` casts. Net negative for an AI-managed codebase.

---

## 4. What we want the app to do

### Port (legacy capabilities to preserve)

- **Landing page** (`Home.tsx`): grid of published strategies, each with a sparkline preview + Sharpe/return/stdev. Click navigates to `/backtest?id=...`.
- **Backtest builder** (`Backtest.tsx` + `Form.tsx`): factor expression editor (Monaco), date range, asset universe, num assets, rebalance interval, start cash. Run → equity curve + holdings + metrics.
- **Factor expression input** (`FactorExpressionInput.tsx`): Monaco editor, factor presets ("momentum", "7-day", etc.), GPT-assisted construction (`/constructFactorEquation`).
- **Streaming backtest** (`/backtest/stream` SSE): per-step progress overlay during long runs. Port the `useBacktestStream` hook 1:1.
- **Benchmark overlay** (`BenchmarkSelector.tsx`): add SPY / QQQ / etc. as comparison lines on the equity chart.
- **Factor snapshot inspector** (`FactorSnapshot.tsx`): click a date on the chart → see holdings + factor scores at that point in time.
- **URL-shareable backtests**: `?id=<strategyID>&start=<date>` deep-link.
- **Bookmarks**: save / unsave a strategy; "your bookmarks" list.
- **Saved strategies**: list of all strategies the user has run.
- **Auth flows**: Google OAuth (cookie session), SMS OTP, sign-out. Port from `auth.tsx`. Cookie auth is the canonical path; the legacy Bearer fallback can be dropped.
- **Investments page** (`Invest.tsx`): list active investments, current value, holdings, completed trades.
- **Invest in strategy** (`InvestInStrategy.tsx`): post a dollar amount against a saved strategy, see it execute via Alpaca.

### Port and improve

- **Errors**: every `alert(err.message)` becomes a toast or inline error card. ESLint already bans `no-alert`.
- **Loading**: every loading state goes through TanStack Query + a consistent skeleton/spinner pattern. No more ad-hoc booleans.
- **Tables**: holdings, trades, snapshots all migrate to TanStack Table with virtualization, tabular numerals, right-aligned numbers, sticky headers.
- **Charts**: every chart that uses `chart.js` migrates to TradingView Lightweight Charts (time series) or Tremor/Recharts (categorical). Add draw-on, gradient fill, crosshair, right-edge price label.
- **Form**: replace the 1,028-line god component with a `useForm` (TanStack Form or React Hook Form) + smaller field components. Hard cap of 700 lines per file enforced by ESLint.
- **Date / number formatting**: one source-of-truth utility for all percentage / currency / date formatting.

### Defer (decide later, do not block scaffolding)

- **Bond ladder page** (`Bond.tsx`, 605 lines). User flagged "minor" but it's real product surface. Open question: port last, or drop entirely?
- **Codegen of TS types from Go API**: `tygo` or hand-port. Hand-port `models.ts` for now; codegen is a future PR.
- **Light theme**: ship dark-only on day one; add light theme once the dark version is locked.
- **Mobile / responsive polish**: target desktop first; mobile gets a "good enough" pass, not a deep redesign, until the desktop UX is settled.

### Drop

- **Bootstrap 5 / react-bootstrap**: gone entirely. shadcn/ui + Tremor cover everything we used it for.
- **`react-confetti-explosion`** and other "fun" deps that never tied to a real workflow.
- **Legacy `userID` cookie** (the homemade UUID-in-cookie thing). Cookie session from `internal/auth` is canonical.
- **`getOrCreateUserID()` writing `HttpOnly` from JS**: that's been silently broken since it was written (`HttpOnly` can only be set server-side).

---

## 5. Quality posture

These are already wired into the scaffold; documenting here so future sessions know the rules without re-reading `eslint.config.js`.

### Lint rules (enforced, `--max-warnings 0` in CI)

- **`max-lines: 700`** per file. Hard structural cap to prevent another `Form.tsx`.
- **`import-x/no-cycle`** — kills circular-import bugs.
- **`no-alert`** — directly forbids the legacy `alert(err.message)` pattern.
- **`no-floating-promises`, `no-misused-promises`** — catches `onClick={asyncFn}` and unawaited promises.
- **`consistent-type-imports`** — enforces `import type` for tree-shaking.
- **`switch-exhaustiveness-check`** — TS unions stay exhaustive.
- **`react-hooks/exhaustive-deps`** as `error` (not warn).
- **`no-explicit-any`**, **`eqeqeq`**, **`prefer-const`**.
- **`no-console`** as warn, allowing only `warn` and `error`.
- TypeScript `strict: true`, `noFallthroughCasesInSwitch`, `noUnusedLocals`, `noUnusedParameters`, `verbatimModuleSyntax`, `forceConsistentCasingInFileNames`, `isolatedModules`.

Style is owned entirely by Prettier (100-col, single quotes, semis, trailing commas). `eslint-config-prettier` disables conflicting style rules.

### CI gate

`.github/workflows/frontend-v2.yml` runs on PRs touching `frontend-v2/**`:
1. `npm run typecheck`
2. `npm run lint`
3. `npm run format:check`
4. `npm run build`

`--max-warnings 0` is the explicit replacement for the legacy `CI=false` hack that suppressed CRA's warnings-as-errors behavior. Warnings are real failures now.

### Things deliberately *not* enforced

- `max-lines-per-function`, `complexity`, `max-depth` — AI writes one-shot complex components and these rules nag without catching real bugs.
- Pre-commit hooks (Husky / lint-staged) — slow AI agents down. CI is the gate.
- Banning raw `fetch` via `no-restricted-syntax` — premature without an `apiClient`. Add when the client exists.

---

## 6. Design references

Open these tabs when designing a screen.

| Product | Steal |
|---|---|
| **TradingView** ([tradingview.com/chart](https://www.tradingview.com/chart/)) | Right-edge floating price label, crosshair with twin axis labels, time-scale density. |
| **Robinhood** ([robinhood.com](https://robinhood.com/)) | Line color flipping green/red vs. session open, laser-mode glowing endpoint, full-bleed equity curve with minimal chrome, scrubbing tooltip that updates the headline number. |
| **Vercel Analytics** ([vercel.com/analytics](https://vercel.com/analytics)) | Gradient area fill recipe, tooltip card style, KPI delta pills (`+12.4%` in green pill), date-range chip selector. |
| **Linear** ([linear.app](https://linear.app)) | Near-black `#08090A` base, hairline borders, RGB-split gradient for hero/empty states, Inter for UI, tight 12-col rhythm. |
| **Stripe Dashboard** ([stripe.com/dashboard](https://stripe.com/)) | Information density without clutter, monospaced numbers right-aligned, sparkline-in-table for holdings. |

Open-source reference codebases to read for shadcn + Tremor patterns: `dub.co`, `cal.com`.

---

## 7. Hosting / deploy story

The Go binary at `cmd/api` will eventually serve `frontend-v2/dist` at `/v2/*` while continuing to serve the legacy `frontend/build` at `/*`. Cutover happens page by page: each new page is approved at `/v2/<page>`, then the legacy route is replaced and the old code deleted.

This is documented but **not yet wired** — that lands with the first PR that needs to deploy a real page.

---

## 8. Open questions / decisions deferred

1. **Bond ladder** (`Bond.tsx`, 605 lines): port last, or drop entirely? Need a call before the rewrite is "complete."
2. **Light theme**: ship at all, or dark-only forever? Decide after the dark version is locked.
3. **Codegen of TS types from Go**: `tygo`, `swag`, hand-port? Punt until it actually hurts.
4. **Mobile polish depth**: "good enough" or deep redesign? Default is "good enough" until desktop is settled.
5. **Zustand** (vs prop-drilling): only adopt if/when a chart and a KPI card both need to react to a hovered date. Don't add preemptively.

---

## 9. What this doc is *not*

- A PR sequence. PRs are scoped at the time they're started, picking from sections 4 and 5 here.
- A timeline. There isn't one.
- Locked. Edit it as direction shifts. The point is to give the next session a running start.
