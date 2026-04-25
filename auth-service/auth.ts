import { betterAuth } from "better-auth";
import { Pool } from "pg";
import { loadConfig } from "./src/config.js";
import { buildPlugins } from "./src/plugins.js";
import { buildDatabaseHooks } from "./src/sync/app-user-profile.js";

const config = loadConfig();

export const pool = new Pool({
  connectionString: config.database.url,
  max: 10,
});

const socialProviders = config.google
  ? {
      google: {
        clientId: config.google.clientId,
        clientSecret: config.google.clientSecret,
        prompt: "select_account" as const,
      },
    }
  : {};

export const auth = betterAuth({
  baseURL: config.baseURL,
  basePath: config.basePath,
  secret: config.secret,
  trustedOrigins: config.trustedOrigins,
  database: pool,
  emailAndPassword: { enabled: false },
  socialProviders,
  plugins: buildPlugins(config),
  databaseHooks: buildDatabaseHooks({ pool, enabled: config.appUserSync.enabled }),
});

export { config };
