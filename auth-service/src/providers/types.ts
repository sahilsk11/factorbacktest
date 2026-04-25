export interface EmailMessage {
  to: string;
  subject: string;
  body: string;
}

export interface EmailSender {
  send(message: EmailMessage): Promise<void>;
}

// Two-method SMS interface so the same shape works for both raw messaging
// (we generate the OTP, Twilio just sends it) and Twilio Verify (Twilio
// generates, sends, and verifies — we never see the code).
export interface SmsService {
  // For raw-messaging providers, `code` is the OTP to send. For Twilio
  // Verify, `code` is ignored — Twilio generates its own.
  send(args: { to: string; code: string }): Promise<void>;

  // Returns true if the code is valid. For raw-messaging providers, this
  // is null (Better Auth verifies internally). For Twilio Verify, this
  // delegates the check to Twilio's API.
  verify?: (args: { to: string; code: string }) => Promise<boolean>;
}
