# frontend-v2

Greenfield rewrite of the factorbacktest frontend. The legacy app at `frontend/` continues
to ship. `frontend-v2/` is replacing it page by page; see
[plans/frontend-v2-north-star.md](../plans/frontend-v2-north-star.md) for the long-running
direction.

## Stack

- Vite + React 19 + TypeScript (strict)
- Tailwind v4 with tokens defined in `src/styles/globals.css` via `@theme`
- Geist Sans + Geist Mono via `@fontsource-variable`
- React Router v7
- TanStack Query for data fetching, behind a typed `apiClient` (`src/lib/api.ts`)
- Framer Motion for entrance animations
- ESLint (flat config) + Prettier

## Configuration

Two env knobs control runtime behavior. Both are respected by `vite dev`,
`vite preview`, production builds, and Playwright (when it lands).

| Variable            | Purpose                                                                                          | Default                                    |
| ------------------- | ------------------------------------------------------------------------------------------------ | ------------------------------------------ |
| `VITE_API_BASE_URL` | Absolute URL of the backend the bundle hits at runtime. Inlined at build time, also read in dev. | `http://localhost:3009` (the local Go API) |
| `PORT`              | Port the dev / preview server binds to. `strictPort: true` — taken ports fail loudly.            | `3000` (matches the Go CORS allowlist)     |

Sources, in priority order:

1. Per-process env (`VITE_API_BASE_URL=https://api.factor.trade npm run dev`)
2. `frontend-v2/.env.local` — personal overrides (gitignored)
3. `frontend-v2/.env` — copy from `.env.example` if you want a local default (gitignored)
4. Built-in defaults in `src/lib/env.ts`

Both `.env` and `.env.local` are gitignored, so a fresh clone runs against the
built-in defaults until you opt in.

## Scripts

```bash
npm run dev           # local FE on :3000 → local Go on :3009
npm run dev:prod-api  # local FE on :3000 → https://api.factor.trade
npm run build         # production build into dist/
npm run preview       # serve the production build locally
npm run typecheck     # tsc project-references typecheck
npm run lint          # eslint, fails on any warning (--max-warnings 0)
npm run lint:fix      # auto-fix what ESLint can
npm run format        # prettier --write .
npm run format:check  # prettier --check . (CI uses this)
```

One-off override pattern:

```bash
PORT=4000 VITE_API_BASE_URL=https://api.factor.trade npm run dev
```

For Playwright, mirror the legacy [frontend/playwright.config.ts](../frontend/playwright.config.ts):

```ts
webServer: [
  {
    command: `npm run build && npm run preview`,
    env: {
      PORT: String(fePort),
      VITE_API_BASE_URL: `http://localhost:${bePort}`,
    },
    url: `http://localhost:${fePort}`,
  },
];
```

## Lint posture

The full rule set lives in `eslint.config.js`; the headline rules:

- **`max-lines: 700`** — hard cap per file. The legacy frontend grew a 1028-line
  `Form.tsx` and we're not doing that again. Combined with `import-x/no-cycle`,
  this is the structural backbone.
- **`--max-warnings 0` in CI** — the AI manages this codebase, so warnings are
  treated as errors at the pipeline boundary. (Replaces the legacy `CI=false` hack
  that disabled CRA's warnings-as-errors behavior.)
- **`no-restricted-syntax: fetch`** — raw `fetch` is forbidden outside
  `src/lib/api.ts`. Every call site goes through `apiClient`.
- **`no-alert`, `no-floating-promises`, `no-misused-promises`,
  `consistent-type-imports`, `react-hooks/exhaustive-deps`** — bug-class rules
  picked specifically to catch patterns the legacy frontend got wrong.
- **TypeScript `strict: true`** plus `noFallthroughCasesInSwitch`,
  `noUnusedLocals`, `noUnusedParameters`, `verbatimModuleSyntax`.

Style is owned entirely by Prettier; ESLint defers to it via `eslint-config-prettier`.

## CI

`.github/workflows/frontend-v2.yml` runs typecheck, lint, format check, and build
on every PR that touches `frontend-v2/**`.
