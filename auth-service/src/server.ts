import { serve } from "@hono/node-server";
import { Hono } from "hono";
import { auth, config, pool } from "../auth.js";

const app = new Hono();

app.get("/api/auth/ok", (c) => c.json({ ok: true }));

app.on(["GET", "POST"], "/api/auth/*", (c) => auth.handler(c.req.raw));

app.notFound((c) =>
  c.json({ error: "not found", path: c.req.path }, 404),
);

const port = config.internalPort;

const server = serve({
  fetch: app.fetch,
  hostname: "127.0.0.1",
  port,
});

console.log(`[auth-service] listening on http://127.0.0.1:${port}${config.basePath}`);

const shutdown = async (signal: string) => {
  console.log(`[auth-service] received ${signal}, shutting down`);
  server.close();
  await pool.end().catch(() => undefined);
  process.exit(0);
};

process.on("SIGTERM", () => void shutdown("SIGTERM"));
process.on("SIGINT", () => void shutdown("SIGINT"));
