// Single source of truth for the API base URL. Keep this file dependency-free
// so it can be imported from anywhere (including auth.tsx) without creating
// circular module loads.
export const endpoint = process.env.REACT_APP_API_PORT
  ? `http://localhost:${process.env.REACT_APP_API_PORT}`
  : process.env.NODE_ENV === "production"
    ? "https://api.factor.trade"
    : "http://localhost:3009";
