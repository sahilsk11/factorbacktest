import type { AuthConfig } from "../config.js";
import { consoleEmailSender } from "./email-console.js";
import { resendEmailSender } from "./email-resend.js";
import { consoleSmsService } from "./sms-console.js";
import { twilioSmsService } from "./sms-twilio.js";
import type { EmailSender, SmsService } from "./types.js";

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

export const buildSmsService = (config: AuthConfig): SmsService => {
  switch (config.sms.provider) {
    case "twilio":
      if (!config.sms.twilio) {
        throw new Error("twilio provider selected but credentials missing");
      }
      return twilioSmsService({
        accountSid: config.sms.twilio.accountSid,
        authToken: config.sms.twilio.authToken,
        from: config.sms.from || undefined,
        messagingServiceSid: config.sms.twilio.messagingServiceSid,
        verifyServiceSid: config.sms.twilio.verifyServiceSid,
      });
    case "console":
    default:
      return consoleSmsService();
  }
};
