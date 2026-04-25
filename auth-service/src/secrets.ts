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

const SECRETS_FILENAMES = ["secrets.json", "secrets-test.json"];

// Walks up from `start` looking for any of the secrets filenames. Same
// strategy as Go's `secretsFileCandidates`, but rooted from the auth-service
// folder so it works whether you `cd auth-service && npm run dev` or run
// the compiled binary out of `/app/auth-service` in the Fly image.
const findSecretsFile = (start: string): string | null => {
  let dir = resolve(start);
  for (let i = 0; i < 6; i++) {
    for (const name of SECRETS_FILENAMES) {
      const candidate = join(dir, name);
      if (existsSync(candidate)) return candidate;
    }
    const parent = dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  return null;
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
      betterAuthSecret: need("betterAuthSecret"),
      googleClientId: process.env.googleClientId,
      googleClientSecret: process.env.googleClientSecret,
      resendApiKey: process.env.resendApiKey,
      twilioAccountSid: process.env.twilioAccountSid,
      twilioAuthToken: process.env.twilioAuthToken,
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
      betterAuthSecret: requireString(auth, "betterAuthSecret", "auth"),
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
export const buildDatabaseUrl = (db: DbSecrets, schema: string): string => {
  const u = new URL("postgres://placeholder/placeholder");
  u.username = encodeURIComponent(db.user);
  u.password = encodeURIComponent(db.password);
  u.hostname = db.host;
  u.port = db.port;
  u.pathname = `/${db.database}`;
  if (db.enableSsl !== false) {
    u.searchParams.set("sslmode", "require");
  } else {
    u.searchParams.set("sslmode", "disable");
  }
  u.searchParams.set("options", `-c search_path=${schema}`);
  return u.toString();
};
