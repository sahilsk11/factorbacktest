import type { EmailSender } from "./types.js";

export const consoleEmailSender = (): EmailSender => ({
  async send({ to, subject, body }) {
    console.log(
      `[email-console] to=${to} subject=${JSON.stringify(subject)} body=${JSON.stringify(body)}`,
    );
  },
});
