// Single source of truth for runtime config. Always-absolute base URL:
// the same code path runs in `vite dev`, `vite preview`, production
// builds, and Playwright. No dev proxy, no NODE_ENV branching.
//
// Resolution order:
//   1. import.meta.env.VITE_API_BASE_URL — inlined at build time, also
//      readable in dev. Set via .env, .env.local, or per-process env
//      (e.g. `npm run dev:prod-api`, or Playwright's webServer.env).
//   2. http://localhost:3009 — the local Go API's default port.
const raw = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '';
const trimmed = raw.replace(/\/$/, '');

export const apiBaseUrl = trimmed || 'http://localhost:3009';
