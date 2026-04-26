// Single source of truth for the API base URL. Keep this file dependency-free
// so it can be imported from anywhere (including auth.tsx) without creating
// circular module loads.
//
// Resolution order (highest priority first):
//   1. REACT_APP_API_URL — full URL override; useful for pointing a local FE
//      at the production API (e.g. REACT_APP_API_URL=https://api.factor.trade)
//      so you can exercise real seed data without standing up the local DB.
//      Production CORS already allows http://localhost:3000 with credentials.
//   2. REACT_APP_API_PORT — local port override (legacy convenience).
//   3. NODE_ENV: 'production' → https://api.factor.trade, otherwise localhost:3009.
export const endpoint = process.env.REACT_APP_API_URL
  ? process.env.REACT_APP_API_URL.replace(/\/$/, "")
  : process.env.REACT_APP_API_PORT
    ? `http://localhost:${process.env.REACT_APP_API_PORT}`
    : process.env.NODE_ENV === "production"
      ? "https://api.factor.trade"
      : "http://localhost:3009";
