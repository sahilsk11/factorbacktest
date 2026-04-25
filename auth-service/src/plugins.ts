import { emailOTP, jwt, phoneNumber } from "better-auth/plugins";
import type { AuthConfig } from "./config.js";
import { buildEmailSender, buildSmsService } from "./providers/index.js";

// Better Auth plugins are heterogeneous; betterAuth() accepts the union.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type AuthPlugin = any;

export const buildPlugins = (config: AuthConfig): AuthPlugin[] => {
  const plugins: AuthPlugin[] = [];

  if (config.features.emailOtp) {
    const email = buildEmailSender(config);
    plugins.push(
      emailOTP({
        otpLength: 6,
        expiresIn: 300,
        allowedAttempts: 3,
        async sendVerificationOTP({ email: to, otp, type }) {
          const subject =
            type === "sign-in"
              ? "Your sign-in code"
              : type === "email-verification"
                ? "Verify your email"
                : "Your password reset code";
          const body = `Your code is: ${otp}\n\nIt expires in 5 minutes.`;
          await email.send({ to, subject, body });
        },
      }),
    );
  }

  if (config.features.smsOtp) {
    const sms = buildSmsService(config);
    // Twilio Verify mode: Twilio generates and validates the OTP. We
    // delegate verification to Twilio via Better Auth's `verifyOTP` hook
    // so we never store SMS codes ourselves.
    const twilioVerifyMode = sms.verify !== undefined;
    plugins.push(
      phoneNumber({
        otpLength: 6,
        expiresIn: 300,
        allowedAttempts: 3,
        async sendOTP({ phoneNumber: to, code }) {
          await sms.send({ to, code });
        },
        ...(twilioVerifyMode
          ? {
              verifyOTP: async ({ phoneNumber: to, code }) => {
                if (!sms.verify) return false;
                return sms.verify({ to, code });
              },
            }
          : {}),
        signUpOnVerification: {
          getTempEmail: (phone) => `${phone.replace(/[^0-9]/g, "")}@phone.local`,
          getTempName: (phone) => phone,
        },
      }),
    );
  }

  plugins.push(jwt());

  return plugins;
};
