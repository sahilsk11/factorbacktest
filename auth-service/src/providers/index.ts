import type { AuthConfig } from "../config.js";
import { consoleEmailSender } from "./email-console.js";
import { resendEmailSender } from "./email-resend.js";
import { consoleSmsSender } from "./sms-console.js";
import { twilioSmsSender } from "./sms-twilio.js";
import type { EmailSender, SmsSender } from "./types.js";

export const buildEmailSender = (config: AuthConfig): EmailSender => {
  switch (config.email.provider) {
    case "resend":
      if (!config.email.resendApiKey) {
        throw new Error("resend provider selected but no api key configured");
      }
      return resendEmailSender(config.email.resendApiKey, config.email.from);
    case "console":
    default:
      return consoleEmailSender();
  }
};

export const buildSmsSender = (config: AuthConfig): SmsSender => {
  switch (config.sms.provider) {
    case "twilio":
      if (!config.sms.twilio) {
        throw new Error("twilio provider selected but credentials missing");
      }
      return twilioSmsSender({
        accountSid: config.sms.twilio.accountSid,
        authToken: config.sms.twilio.authToken,
        from: config.sms.from,
        messagingServiceSid: config.sms.twilio.messagingServiceSid,
      });
    case "console":
    default:
      return consoleSmsSender();
  }
};
