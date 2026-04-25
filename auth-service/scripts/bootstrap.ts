import { Client } from "pg";
import { loadConfig } from "../src/config.js";

// Inlined so the compiled JS doesn't need a sibling .sql file at runtime.
// Mirrors scripts/bootstrap-schema.sql (kept in the repo as documentation).
const BOOTSTRAP_SQL = `
CREATE SCHEMA IF NOT EXISTS "__SCHEMA__";

CREATE TABLE IF NOT EXISTS public.app_user_profile (
    auth_user_id  TEXT PRIMARY KEY,
    email         TEXT,
    display_name  TEXT,
    phone_number  TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS app_user_profile_email_idx
    ON public.app_user_profile (email);
CREATE INDEX IF NOT EXISTS app_user_profile_phone_idx
    ON public.app_user_profile (phone_number);
`;

const main = async () => {
  const config = loadConfig();

  const adminUrl = new URL(config.database.url);
  adminUrl.searchParams.delete("options");

  const schema = config.database.schema;
  if (!/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(schema)) {
    throw new Error(`unsafe schema name: ${schema}`);
  }
  const sql = BOOTSTRAP_SQL.replace("__SCHEMA__", schema);

  const client = new Client({ connectionString: adminUrl.toString() });
  await client.connect();
  try {
    await client.query(sql);
    console.log(`[bootstrap] ensured schema "${schema}" + public.app_user_profile exist`);
  } finally {
    await client.end();
  }
};

main().catch((err) => {
  console.error("[bootstrap] failed", err);
  process.exit(1);
});
