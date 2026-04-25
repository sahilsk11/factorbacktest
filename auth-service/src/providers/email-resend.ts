import { Resend } from "resend";
import type { EmailSender } from "./types.js";

export const resendEmailSender = (apiKey: string, from: string): EmailSender => {
  const client = new Resend(apiKey);
  return {
    async send({ to, subject, body }) {
      const { error } = await client.emails.send({
        from,
        to,
        subject,
        text: body,
      });
      if (error) {
        throw new Error(`resend send failed: ${error.message ?? JSON.stringify(error)}`);
      }
    },
  };
};
