import type { SmsService } from "./types.js";

// Dev-only SMS service: prints OTPs to stdout instead of sending real SMS.
// Better Auth still generates and verifies the codes itself, so no `verify`
// hook is needed.
export const consoleSmsService = (): SmsService => ({
  async send({ to, code }) {
    console.log(`[sms-console] to=${to} code=${code}`);
  },
});
