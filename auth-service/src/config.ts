import { z } from "zod";

const featureFlag = (raw: string | undefined, fallback: boolean): boolean => {
  if (raw === undefined) return fallback;
  return ["1", "true", "yes", "on"].includes(raw.toLowerCase());
};

const envSchema = z.object({
  APP_BASE_URL: z.string().url(),
  AUTH_BASE_PATH: z.string().default("/api/auth"),
  AUTH_INTERNAL_PORT: z.coerce.number().int().default(3001),
  TRUSTED_ORIGINS: z.string().optional(),

  BETTER_AUTH_SECRET: z.string().min(32, "BETTER_AUTH_SECRET must be at least 32 chars"),

  DATABASE_URL: z.string().url(),
  AUTH_DB_SCHEMA: z.string().default("auth"),

  FEATURE_GOOGLE: z.string().optional(),
  FEATURE_EMAIL_OTP: z.string().optional(),
  FEATURE_SMS_OTP: z.string().optional(),

  GOOGLE_CLIENT_ID: z.string().optional(),
  GOOGLE_CLIENT_SECRET: z.string().optional(),

  EMAIL_PROVIDER: z.enum(["console", "resend"]).default("console"),
  EMAIL_FROM: z.string().optional(),
  RESEND_API_KEY: z.string().optional(),

  SMS_PROVIDER: z.enum(["console", "twilio"]).default("console"),
  SMS_FROM: z.string().optional(),
  TWILIO_ACCOUNT_SID: z.string().optional(),
  TWILIO_AUTH_TOKEN: z.string().optional(),
  TWILIO_MESSAGING_SERVICE_SID: z.string().optional(),

  APP_USER_SYNC_ENABLED: z.string().optional(),
});

export type AuthEnv = z.infer<typeof envSchema>;

export interface AuthConfig {
  baseURL: string;
  basePath: string;
  internalPort: number;
  secret: string;
  trustedOrigins: string[];
  database: {
    url: string;
    schema: string;
  };
  features: {
    google: boolean;
    emailOtp: boolean;
    smsOtp: boolean;
  };
  google?: {
    clientId: string;
    clientSecret: string;
  };
  email: {
    provider: "console" | "resend";
    from: string;
    resendApiKey?: string;
  };
  sms: {
    provider: "console" | "twilio";
    from: string;
    twilio?: {
      accountSid: string;
      authToken: string;
      messagingServiceSid?: string;
    };
  };
  appUserSync: {
    enabled: boolean;
  };
}

const ensureSearchPath = (rawUrl: string, schema: string): string => {
  const u = new URL(rawUrl);
  const existing = u.searchParams.get("options");
  const directive = `-c search_path=${schema}`;
  if (existing) {
    if (existing.includes("search_path=")) return u.toString();
    u.searchParams.set("options", `${existing} ${directive}`);
  } else {
    u.searchParams.set("options", directive);
  }
  return u.toString();
};

export const loadConfig = (): AuthConfig => {
  const env = envSchema.parse(process.env);

  const features = {
    google: featureFlag(env.FEATURE_GOOGLE, true),
    emailOtp: featureFlag(env.FEATURE_EMAIL_OTP, true),
    smsOtp: featureFlag(env.FEATURE_SMS_OTP, true),
  };

  if (features.google && (!env.GOOGLE_CLIENT_ID || !env.GOOGLE_CLIENT_SECRET)) {
    throw new Error(
      "FEATURE_GOOGLE is on but GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET are not set",
    );
  }
  if (
    (features.emailOtp || features.smsOtp) &&
    env.EMAIL_PROVIDER === "resend" &&
    !env.RESEND_API_KEY
  ) {
    throw new Error("EMAIL_PROVIDER=resend requires RESEND_API_KEY");
  }
  if (features.smsOtp && env.SMS_PROVIDER === "twilio") {
    if (!env.TWILIO_ACCOUNT_SID || !env.TWILIO_AUTH_TOKEN) {
      throw new Error(
        "SMS_PROVIDER=twilio requires TWILIO_ACCOUNT_SID and TWILIO_AUTH_TOKEN",
      );
    }
  }

  const trustedOrigins = (env.TRUSTED_ORIGINS ?? env.APP_BASE_URL)
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);

  const databaseUrl = ensureSearchPath(env.DATABASE_URL, env.AUTH_DB_SCHEMA);

  return {
    baseURL: env.APP_BASE_URL,
    basePath: env.AUTH_BASE_PATH,
    internalPort: env.AUTH_INTERNAL_PORT,
    secret: env.BETTER_AUTH_SECRET,
    trustedOrigins,
    database: {
      url: databaseUrl,
      schema: env.AUTH_DB_SCHEMA,
    },
    features,
    google:
      features.google && env.GOOGLE_CLIENT_ID && env.GOOGLE_CLIENT_SECRET
        ? {
            clientId: env.GOOGLE_CLIENT_ID,
            clientSecret: env.GOOGLE_CLIENT_SECRET,
          }
        : undefined,
    email: {
      provider: env.EMAIL_PROVIDER,
      from: env.EMAIL_FROM ?? "no-reply@localhost",
      resendApiKey: env.RESEND_API_KEY,
    },
    sms: {
      provider: env.SMS_PROVIDER,
      from: env.SMS_FROM ?? "",
      twilio:
        env.SMS_PROVIDER === "twilio" &&
        env.TWILIO_ACCOUNT_SID &&
        env.TWILIO_AUTH_TOKEN
          ? {
              accountSid: env.TWILIO_ACCOUNT_SID,
              authToken: env.TWILIO_AUTH_TOKEN,
              messagingServiceSid: env.TWILIO_MESSAGING_SERVICE_SID,
            }
          : undefined,
    },
    appUserSync: {
      enabled: featureFlag(env.APP_USER_SYNC_ENABLED, true),
    },
  };
};
