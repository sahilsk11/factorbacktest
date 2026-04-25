import twilio from "twilio";
import type { SmsSender } from "./types.js";

export interface TwilioSmsOptions {
  accountSid: string;
  authToken: string;
  from: string;
  messagingServiceSid?: string;
}

export const twilioSmsSender = (opts: TwilioSmsOptions): SmsSender => {
  const client = twilio(opts.accountSid, opts.authToken);
  return {
    async send({ to, body }) {
      const params: Parameters<typeof client.messages.create>[0] = {
        to,
        body,
      };
      if (opts.messagingServiceSid) {
        params.messagingServiceSid = opts.messagingServiceSid;
      } else {
        params.from = opts.from;
      }
      await client.messages.create(params);
    },
  };
};
