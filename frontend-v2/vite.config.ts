import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// Single source of truth for runtime config. Both `vite` (dev) and
// `vite preview` (production-realistic local server, also used by
// Playwright) bind to PORT with strictPort:true so a taken port fails
// loudly instead of silently incrementing — Playwright relies on the
// port it picked actually being the port the server bound to.
//
// The backend URL is *not* read here. It's read by the running app via
// import.meta.env.VITE_API_BASE_URL so the same bundle can be pointed
// at any backend without rebuilding.
const port = Number(process.env.PORT ?? 3000);

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { tsconfigPaths: true },
  server: { port, strictPort: true, host: true },
  preview: { port, strictPort: true, host: true },
});
