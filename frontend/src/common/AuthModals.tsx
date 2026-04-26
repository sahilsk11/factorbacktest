import { useState } from "react";
import { useAuth } from "auth";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import PhoneInput, { isValidPhoneNumber } from "react-phone-number-input/input";

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { AlertCircle } from "lucide-react";

type PageState = "initial" | "smsConfirmation";

export default function LoginModal({
  show,
  close,
  onSuccess,
}: {
  show: boolean;
  close: () => void;
  onSuccess?: () => void;
}) {
  const { signIn } = useAuth();
  const [pageState, setPageState] = useState<PageState>("initial");
  const [phoneNumber, setPhoneNumber] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  const handleGoogle = async () => {
    if (loading) return;
    setError("");
    setLoading(true);
    try {
      await signIn.google();
      onSuccess?.();
      close();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Google sign-in failed");
    } finally {
      setLoading(false);
    }
  };

  const handleSmsRequest = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      await signIn.sendSmsOtp(phoneNumber);
      setPageState("smsConfirmation");
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to send SMS code");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={show}
      onOpenChange={(open) => {
        if (!open) {
          setPageState("initial");
          setError("");
          close();
        }
      }}
    >
      <DialogContent className="sm:max-w-[425px] grid gap-4 p-10 pt-3">
        <DialogHeader className="text-left">
          <DialogTitle className="mb-0 text-center">Login</DialogTitle>
        </DialogHeader>

        {error && (
          <Alert className="pb-3 pt-0" variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertTitle className="mt-2 text-lg">Error</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {pageState === "initial" && (
          <InitialPanel
            phoneNumber={phoneNumber}
            setPhoneNumber={setPhoneNumber}
            onGoogle={handleGoogle}
            onSmsRequest={handleSmsRequest}
            loading={loading}
          />
        )}

        {pageState === "smsConfirmation" && (
          <SmsConfirmationPanel
            phoneNumber={phoneNumber}
            setError={setError}
            onSuccess={() => {
              onSuccess?.();
              close();
            }}
          />
        )}

        {loading && <Spinner />}
      </DialogContent>
    </Dialog>
  );
}

function Spinner() {
  return (
    <div className="flex justify-center">
      <div className="animate-caret">
        <span className="sr-only">Loading...</span>
        <svg className="h-6 w-6 animate-spin fill-primary-foreground" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path
            className="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
      </div>
    </div>
  );
}

function InitialPanel({
  phoneNumber,
  setPhoneNumber,
  onGoogle,
  onSmsRequest,
  loading,
}: {
  phoneNumber: string;
  setPhoneNumber: (v: any) => void;
  onGoogle: () => void;
  onSmsRequest: (e: React.FormEvent<HTMLFormElement>) => void;
  loading: boolean;
}) {
  return (
    <>
      <Button onClick={onGoogle} type="button" className="w-full" disabled={loading}>
        Continue with Google
      </Button>

      <Divider label="or phone" />

      <form onSubmit={onSmsRequest}>
        <div className="grid gap-3">
          <div className="grid gap-2">
            <Label>Phone Number</Label>
            <PhoneInput
              placeholder="(408) 555-1234"
              country="US"
              className={cn(
                "flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
              )}
              required
              value={phoneNumber}
              onChange={setPhoneNumber}
            />
          </div>
          <Button
            disabled={loading || !isValidPhoneNumber(phoneNumber || "", "US")}
            type="submit"
            className="w-full"
          >
            Text me a code
          </Button>
        </div>
      </form>
    </>
  );
}

function Divider({ label }: { label: string }) {
  return (
    <div className="relative">
      <div className="absolute inset-0 flex items-center">
        <span className="w-full border-t" />
      </div>
      <div className="relative flex justify-center text-xs uppercase">
        <span className="bg-background px-2 text-muted-foreground">{label}</span>
      </div>
    </div>
  );
}

function SmsConfirmationPanel({
  phoneNumber,
  setError,
  onSuccess,
}: {
  phoneNumber: string;
  setError: React.Dispatch<React.SetStateAction<string>>;
  onSuccess: () => void;
}) {
  const { signIn } = useAuth();
  const [code, setCode] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const onSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (submitting) return;
    setSubmitting(true);
    try {
      await signIn.verifySmsOtp(phoneNumber, code);
      onSuccess();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Invalid code");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={onSubmit}>
      <div className="grid gap-4">
        <div className="grid gap-2">
          <Label>SMS Confirmation Code</Label>
          <Input
            autoFocus
            required
            type="number"
            value={code}
            onChange={(e) => {
              setError("");
              setCode(e.target.value);
            }}
          />
        </div>
        <Button disabled={submitting || code.length !== 6} type="submit" className="w-full">
          Continue
        </Button>
      </div>
    </form>
  );
}
