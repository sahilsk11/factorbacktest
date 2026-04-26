import { existsSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";

// Mirrors the Go-side `Secrets` struct in internal/util/util.go. Only the
// fields the auth-service actually consumes are typed here; the rest are
// allowed to exist in the JSON without complaint.
export interface DbSecrets {
  host: string;
  port: string;
  user: string;
  password: string;
  database: string;
  enableSsl?: boolean;
}

export interface AuthSecrets {
  betterAuthSecret: string;
  googleClientId?: string;
  googleClientSecret?: string;
  resendApiKey?: string;
  twilioAccountSid?: string;
  twilioAuthToken?: string;
  // The Twilio Verify Service SID. Required when running SMS OTP through
  // Twilio Verify (the recommended mode — Twilio handles code generation,
  // delivery, retry, and fraud protection; we never see or store the code).
  twilioVerifyServiceSid?: string;
}

export interface ResolvedSecrets {
  db: DbSecrets;
  auth: AuthSecrets;
}

interface SecretsFile {
  db?: Partial<DbSecrets>;
  auth?: Partial<AuthSecrets>;
  // Other top-level keys (`gpt`, `jwt`, `alpaca`, etc.) are tolerated but
  // ignored here — they belong to the Go side.
  [k: string]: unknown;
}

// Mirrors the Go-side `secretsFileCandidates` selector: ALPHA_ENV picks
// the file. Default order (no env set) prefers prod `secrets.json`, with
// `secrets-test.json` as a fallback for fresh checkouts.
const candidatesForEnv = (env: string | undefined): string[] => {
  switch (env) {
    case "dev":
      return ["secrets-dev.json", "secrets.json"];
    case "test":
      return ["secrets-test.json"];
    case "prod":
      return ["secrets.json"];
    default:
      return ["secrets.json", "secrets-test.json"];
  }
};

// Walks up from `start` looking for any of the secrets filenames. Same
// strategy as Go's `secretsFileCandidates`, but rooted from the auth-service
// folder so it works whether you `cd auth-service && npm run dev` or run
// the compiled binary out of `/app/auth-service` in the Fly image.
const findSecretsFile = (start: string): string | null => {
  const filenames = candidatesForEnv(process.env.ALPHA_ENV);
  let dir = resolve(start);
  for (let i = 0; i < 6; i++) {
    for (const name of filenames) {
      const candidate = join(dir, name);
      if (existsSync(candidate)) return candidate;
    }
    const parent = dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  return null;
};

// Minimum entropy for the Better Auth signing secret. 32 chars matches what
// `openssl rand -hex 16` produces (16 bytes); a hex-encoded 32-byte secret
// is 64 chars. We pick 32 as a conservative floor.
const MIN_BETTER_AUTH_SECRET_LEN = 32;

const validateBetterAuthSecret = (s: string): string => {
  if (s.length < MIN_BETTER_AUTH_SECRET_LEN) {
    throw new Error(
      `betterAuthSecret is too short (${s.length} chars). ` +
        `Use at least ${MIN_BETTER_AUTH_SECRET_LEN} characters; ` +
        `generate with \`openssl rand -hex 32\`.`,
    );
  }
  return s;
};

const loadFromEnv = (): ResolvedSecrets => {
  const need = (name: string): string => {
    const v = process.env[name];
    if (!v) {
      throw new Error(
        `FB_SECRETS_FROM_ENV=1 but env var "${name}" is not set`,
      );
    }
    return v;
  };
  return {
    db: {
      host: need("host"),
      port: need("port"),
      user: need("user"),
      password: need("password"),
      database: need("database"),
      enableSsl: process.env.enableSsl
        ? process.env.enableSsl !== "false"
        : true,
    },
    auth: {
      betterAuthSecret: validateBetterAuthSecret(need("betterAuthSecret")),
      googleClientId: process.env.googleClientId,
      googleClientSecret: process.env.googleClientSecret,
      resendApiKey: process.env.resendApiKey,
      twilioAccountSid: process.env.twilioAccountSid,
      twilioAuthToken: process.env.twilioAuthToken,
      twilioVerifyServiceSid: process.env.twilioVerifyServiceSid,
    },
  };
};

const loadFromFile = (path: string): ResolvedSecrets => {
  const raw = readFileSync(path, "utf8");
  const parsed = JSON.parse(raw) as SecretsFile;
  if (!parsed.db) {
    throw new Error(`secrets file ${path} is missing the "db" section`);
  }
  if (!parsed.auth) {
    throw new Error(
      `secrets file ${path} is missing the "auth" section. Add at minimum: { "betterAuthSecret": "<32 byte hex>" }`,
    );
  }
  const requireString = (
    obj: Record<string, unknown>,
    key: string,
    section: string,
  ): string => {
    const v = obj[key];
    if (typeof v !== "string" || v.length === 0) {
      throw new Error(`secrets file ${path}: missing ${section}.${key}`);
    }
    return v;
  };
  const db = parsed.db as Record<string, unknown>;
  const auth = parsed.auth as Record<string, unknown>;
  return {
    db: {
      host: requireString(db, "host", "db"),
      port: requireString(db, "port", "db"),
      user: requireString(db, "user", "db"),
      password: requireString(db, "password", "db"),
      database: requireString(db, "database", "db"),
      enableSsl: typeof db.enableSsl === "boolean" ? db.enableSsl : true,
    },
    auth: {
      betterAuthSecret: validateBetterAuthSecret(
        requireString(auth, "betterAuthSecret", "auth"),
      ),
      googleClientId:
        typeof auth.googleClientId === "string" ? auth.googleClientId : undefined,
      googleClientSecret:
        typeof auth.googleClientSecret === "string"
          ? auth.googleClientSecret
          : undefined,
      resendApiKey:
        typeof auth.resendApiKey === "string" ? auth.resendApiKey : undefined,
      twilioAccountSid:
        typeof auth.twilioAccountSid === "string"
          ? auth.twilioAccountSid
          : undefined,
      twilioAuthToken:
        typeof auth.twilioAuthToken === "string"
          ? auth.twilioAuthToken
          : undefined,
      twilioVerifyServiceSid:
        typeof auth.twilioVerifyServiceSid === "string"
          ? auth.twilioVerifyServiceSid
          : undefined,
    },
  };
};

export const loadSecrets = (cwd = process.cwd()): ResolvedSecrets => {
  if (process.env.FB_SECRETS_FROM_ENV === "1") {
    return loadFromEnv();
  }
  const path = findSecretsFile(cwd);
  if (!path) {
    throw new Error(
      `could not find secrets.json (searched up from ${cwd}). ` +
        `Either create one (see secrets-test.json for the shape) or set FB_SECRETS_FROM_ENV=1.`,
    );
  }
  return loadFromFile(path);
};

// Builds a Postgres connection string with `?options=-c search_path=<schema>`
// applied so Better Auth lands tables in the correct schema.
//
// SSL policy is set on the pg `Client` / `Pool` constructor via
// `buildPoolSslOption` (which overrides anything parsed from the URL), so
// the only `sslmode` we encode here is `disable` for envs that opt out of
// TLS entirely (i.e. local dev against Docker Postgres). Encoding
// `sslmode=require` in the URL would also work in prod but produces a
// migration warning from pg-connection-string@2.12+, so we keep the policy
// in one place — the Pool config — instead.
export const buildDatabaseUrl = (db: DbSecrets, schema: string): string => {
  // Defense in depth: even though zod typechecks `schema` as a string, the
  // value is interpolated directly into the connection string's `options`
  // parameter, which Postgres parses as a server-side SET command. A weird
  // value here could surprise the search_path or break the connection.
  if (!/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(schema)) {
    throw new Error(`unsafe AUTH_DB_SCHEMA: ${schema}`);
  }
  const u = new URL("postgres://placeholder/placeholder");
  u.username = encodeURIComponent(db.user);
  u.password = encodeURIComponent(db.password);
  u.hostname = db.host;
  u.port = db.port;
  u.pathname = `/${db.database}`;
  if (db.enableSsl === false) {
    u.searchParams.set("sslmode", "disable");
  }
  u.searchParams.set("options", `-c search_path=${schema}`);
  return u.toString();
};

// Returns the `ssl` option to pass to pg `Client` / `Pool`.
//
// - `enableSsl !== false` (prod default): use TLS but do NOT verify the
//   certificate chain. This matches the Go side, which connects with
//   libpq's `sslmode=prefer` semantics (encrypt, don't verify). Strict
//   verification would fail with `CERT_HAS_EXPIRED` whenever the upstream
//   Postgres provider's chain doesn't validate against Node's bundled CAs,
//   which has happened in prod.
// - `enableSsl === false` (local Docker Postgres): no TLS at all.
export const buildPoolSslOption = (
  db: DbSecrets,
): { rejectUnauthorized: false } | false => {
  if (db.enableSsl === false) return false;
  return { rejectUnauthorized: false };
};
