import { z } from "zod";
import { buildDatabaseUrl, loadSecrets } from "./secrets.js";

// Non-secret configuration. All values default to something sensible for
// local dev so a fresh checkout with a populated `secrets.json` "just works".
// Production overrides go in fly.toml's [env] block.
const flag = (raw: string | undefined, fallback: boolean): boolean => {
  if (raw === undefined) return fallback;
  return ["1", "true", "yes", "on"].includes(raw.toLowerCase());
};

const envSchema = z.object({
  APP_BASE_URL: z.string().url().default("http://localhost:3009"),
  AUTH_BASE_PATH: z.string().default("/api/auth"),
  AUTH_INTERNAL_PORT: z.coerce.number().int().default(3001),
  TRUSTED_ORIGINS: z.string().optional(),

  AUTH_DB_SCHEMA: z.string().default("auth"),

  FEATURE_GOOGLE: z.string().optional(),
  FEATURE_EMAIL_OTP: z.string().optional(),
  FEATURE_SMS_OTP: z.string().optional(),

  EMAIL_PROVIDER: z.enum(["console", "resend"]).default("console"),
  EMAIL_FROM: z.string().default("no-reply@localhost"),

  SMS_PROVIDER: z.enum(["console", "twilio"]).default("console"),
  SMS_FROM: z.string().default(""),
  TWILIO_MESSAGING_SERVICE_SID: z.string().optional(),

  APP_USER_SYNC_ENABLED: z.string().optional(),
});

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
      verifyServiceSid?: string;
    };
  };
  appUserSync: {
    enabled: boolean;
  };
}

export const loadConfig = (): AuthConfig => {
  const env = envSchema.parse(process.env);
  const secrets = loadSecrets();

  // Default Google off locally because most dev machines don't have OAuth
  // credentials configured. Production fly.toml flips it on explicitly.
  const features = {
    google: flag(env.FEATURE_GOOGLE, false),
    emailOtp: flag(env.FEATURE_EMAIL_OTP, true),
    smsOtp: flag(env.FEATURE_SMS_OTP, true),
  };

  const google =
    features.google && secrets.auth.googleClientId && secrets.auth.googleClientSecret
      ? {
          clientId: secrets.auth.googleClientId,
          clientSecret: secrets.auth.googleClientSecret,
        }
      : undefined;
  if (features.google && !google) {
    throw new Error(
      "FEATURE_GOOGLE=true but secrets.auth.googleClientId / googleClientSecret are not set",
    );
  }

  if (env.EMAIL_PROVIDER === "resend" && !secrets.auth.resendApiKey) {
    throw new Error("EMAIL_PROVIDER=resend requires secrets.auth.resendApiKey");
  }
  if (env.SMS_PROVIDER === "twilio") {
    if (!secrets.auth.twilioAccountSid || !secrets.auth.twilioAuthToken) {
      throw new Error(
        "SMS_PROVIDER=twilio requires secrets.auth.twilioAccountSid and secrets.auth.twilioAuthToken",
      );
    }
  }

  const trustedOrigins = (
    env.TRUSTED_ORIGINS ?? `${env.APP_BASE_URL},http://localhost:3000`
  )
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);

  return {
    baseURL: env.APP_BASE_URL,
    basePath: env.AUTH_BASE_PATH,
    internalPort: env.AUTH_INTERNAL_PORT,
    secret: secrets.auth.betterAuthSecret,
    trustedOrigins,
    database: {
      url: buildDatabaseUrl(secrets.db, env.AUTH_DB_SCHEMA),
      schema: env.AUTH_DB_SCHEMA,
    },
    features,
    google,
    email: {
      provider: env.EMAIL_PROVIDER,
      from: env.EMAIL_FROM,
      resendApiKey: secrets.auth.resendApiKey,
    },
    sms: {
      provider: env.SMS_PROVIDER,
      from: env.SMS_FROM,
      twilio:
        env.SMS_PROVIDER === "twilio" &&
        secrets.auth.twilioAccountSid &&
        secrets.auth.twilioAuthToken
          ? {
              accountSid: secrets.auth.twilioAccountSid,
              authToken: secrets.auth.twilioAuthToken,
              messagingServiceSid: env.TWILIO_MESSAGING_SERVICE_SID,
              verifyServiceSid: secrets.auth.twilioVerifyServiceSid,
            }
          : undefined,
    },
    appUserSync: {
      enabled: flag(env.APP_USER_SYNC_ENABLED, false),
    },
  };
};
