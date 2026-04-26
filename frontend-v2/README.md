# frontend-v2

Greenfield rewrite of the factorbacktest frontend. **Scaffolding only — no implementation yet.**

The legacy app at `frontend/` continues to ship. `frontend-v2/` will replace it page by
page in subsequent PRs.

## Stack

- Vite + React + TypeScript
- ESLint (flat config) + Prettier
- The styling system, component library, charting library, data layer, and routing
  are deliberately **not** wired in yet — each lands with the PR that needs it.

## Scripts

```bash
npm run dev           # local dev server
npm run build         # production build into dist/
npm run preview       # serve the production build locally
npm run typecheck     # tsc project-references typecheck
npm run lint          # eslint, fails on any warning (--max-warnings 0)
npm run lint:fix      # auto-fix what ESLint can
npm run format        # prettier --write .
npm run format:check  # prettier --check . (CI uses this)
```

## Lint posture

The full rule set lives in `eslint.config.js`; the headline rules:

- **`max-lines: 700`** — hard cap per file. The legacy frontend grew a 1028-line
  `Form.tsx` and we're not doing that again. Combined with `import-x/no-cycle`,
  this is the structural backbone.
- **`--max-warnings 0` in CI** — the AI manages this codebase, so warnings are
  treated as errors at the pipeline boundary. (This replaces the legacy
  `CI=false` hack that disabled CRA's warnings-as-errors behavior.)
- **`no-alert`, `no-floating-promises`, `no-misused-promises`,
  `consistent-type-imports`, `react-hooks/exhaustive-deps`** — bug-class rules
  picked specifically to catch patterns the legacy frontend got wrong.
- **TypeScript `strict: true`** plus `noFallthroughCasesInSwitch`,
  `noUnusedLocals`, `noUnusedParameters`, `verbatimModuleSyntax`.

Style is owned entirely by Prettier; ESLint defers to it via `eslint-config-prettier`.

## CI

`.github/workflows/frontend-v2.yml` runs typecheck, lint, format check, and build
on every PR that touches `frontend-v2/**`.
