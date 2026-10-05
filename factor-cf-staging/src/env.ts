export interface Env {
	FACTOR_API: DurableObjectNamespace;
	/** Same Neon URL as Fly prod (set via wrangler secret; never commit). */
	DATABASE_URL: string;
	MIGRATE_DATABASE_URL?: string;
	CRON_SECRET: string;
	ADMIN_API_KEY?: string;
	APP_BASE_URL: string;
	FACTOR_AUTH_FRONTEND_BASE_URL: string;
	dataJockey: string;
	gpt: string;
	host?: string;
	port?: string;
	user?: string;
	password?: string;
	database?: string;
	enableSsl?: string;
	apiKey: string;
	apiSecret: string;
	endpoint: string;
	sessionSecret?: string;
	googleClientId?: string;
	googleClientSecret?: string;
	twilioAccountSid?: string;
	twilioAuthToken?: string;
	twilioVerifyServiceSid?: string;
	resend_apiKey?: string;
	resend_fromEmail?: string;
	resend_fromName?: string;
	region?: string;
	fromEmail?: string;
}

/** Maps Worker secret bindings → process env expected by FB_SECRETS_FROM_ENV=1 (Fly parity). */
export function factorContainerEnv(env: Env): Record<string, string> {
	const out: Record<string, string> = {
		FB_SECRETS_FROM_ENV: "1",
		GIN_MODE: "release",
		TZ: "America/New_York",
	};

	const copy = (
		key: keyof Env & string,
		target: string = key,
	): void => {
		const v = env[key];
		if (typeof v === "string" && v.length > 0) {
			out[target] = v;
		}
	};

	copy("DATABASE_URL");
	copy("MIGRATE_DATABASE_URL");
	copy("dataJockey");
	copy("gpt");
	copy("host");
	copy("port");
	copy("user");
	copy("password");
	copy("database");
	copy("enableSsl");
	copy("apiKey");
	copy("apiSecret");
	copy("endpoint");
	copy("sessionSecret");
	copy("googleClientId");
	copy("googleClientSecret");
	copy("twilioAccountSid");
	copy("twilioAuthToken");
	copy("twilioVerifyServiceSid");
	copy("resend_apiKey");
	copy("resend_fromEmail");
	copy("resend_fromName");
	copy("region");
	copy("fromEmail");
	copy("CRON_SECRET");
	copy("ADMIN_API_KEY");
	copy("APP_BASE_URL");
	copy("FACTOR_AUTH_FRONTEND_BASE_URL");

	return out;
}
