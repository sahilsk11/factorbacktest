import twilio from "twilio";
import type { SmsService } from "./types.js";

export interface TwilioSmsOptions {
  accountSid: string;
  authToken: string;
  from?: string;
  messagingServiceSid?: string;
  // When set, run SMS OTP through Twilio Verify instead of raw messaging.
  // Twilio generates the code, sends it, validates it, and protects against
  // SIM-swap / abuse. Recommended.
  verifyServiceSid?: string;
}

export const twilioSmsService = (opts: TwilioSmsOptions): SmsService => {
  const client = twilio(opts.accountSid, opts.authToken);

  if (opts.verifyServiceSid) {
    const service = client.verify.v2.services(opts.verifyServiceSid);
    return {
      async send({ to }) {
        // Twilio Verify generates and sends its own code.
        await service.verifications.create({ to, channel: "sms" });
      },
      async verify({ to, code }) {
        // Twilio throws on transport/auth/rate errors. Treat any error
        // here as "could not verify" so Better Auth returns a clean 4xx
        // INVALID_OTP instead of leaking a 5xx + stack to the client.
        // We still log so ops can distinguish "wrong code" from "Twilio
        // is broken / rate-limited" in production logs.
        try {
          const result = await service.verificationChecks.create({ to, code });
          return result.status === "approved";
        } catch (err) {
          const status =
            (err as { status?: number; code?: number }).status ??
            (err as { code?: number }).code;
          console.error(
            `[sms-twilio] verificationChecks.create failed (status=${status}):`,
            err,
          );
          return false;
        }
      },
    };
  }

  // Fallback: raw messaging — caller (Better Auth) generates the code.
  return {
    async send({ to, code }) {
      const params: Parameters<typeof client.messages.create>[0] = {
        to,
        body: `Your code is: ${code}`,
      };
      if (opts.messagingServiceSid) {
        params.messagingServiceSid = opts.messagingServiceSid;
      } else if (opts.from) {
        params.from = opts.from;
      } else {
        throw new Error("twilio raw messaging requires `from` or `messagingServiceSid`");
      }
      await client.messages.create(params);
    },
  };
};
