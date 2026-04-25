import type { SmsSender } from "./types.js";

export const consoleSmsSender = (): SmsSender => ({
  async send({ to, body }) {
    console.log(`[sms-console] to=${to} body=${JSON.stringify(body)}`);
  },
});
