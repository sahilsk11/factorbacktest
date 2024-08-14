import { useState } from "react";
import { useAuth } from "auth";
import modalsStyle from "./Modals.module.css";
import { GoogleLogin } from "@react-oauth/google";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import PhoneInput, { isValidPhoneNumber } from "react-phone-number-input/input";

import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { SupabaseClient } from "@supabase/supabase-js";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { AlertCircle } from "lucide-react";

export default function LoginModal({
  show,
  close,
  onSuccess
}: { show: boolean, close: () => void, onSuccess?: () => void }) {
  // create enum for page
  type PageState = "initial" | "phoneConfirmation"
  const { supabase } = useAuth()
  const [phoneNumber, setPhoneNumber] = useState<any>("");
  const [pageState, setPageState] = useState<PageState>("initial");
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  if (!supabase) {
    return null;
  }

  const handleFormSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setLoading(true);
    setError("");
    const { data, error } = await supabase.auth.signInWithOtp({
      phone: phoneNumber,
    })
    if (error) {
      setError(error.message)
      setLoading(false)
    } else {
      setLoading(false)
      setPageState("phoneConfirmation")
    }
  }

  return (
    <Dialog open={show} onOpenChange={(c) => { if (!c) { close() } }}>
      <DialogContent className="sm:max-w-[425px] grid gap-4 p-10 pt-3">
        <DialogHeader className="text-left">
          <DialogTitle className="mb-0">Login</DialogTitle>
          {/* <CardDescription>
              Use your phone number or Google account.
            </CardDescription> */}
        </DialogHeader>

        {error ?
          <Alert className="pb-3 pt-0" variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertTitle className="mt-2 text-lg">Error</AlertTitle>
            <AlertDescription>
              {error}
            </AlertDescription>
          </Alert>
          : null}
        {pageState === "initial" ? <InitialLoginDialog
          supabase={supabase}
          handleFormSubmit={handleFormSubmit}
          phoneNumber={phoneNumber}
          setPhoneNumber={setPhoneNumber}
          close={close}
          onSuccess={onSuccess}
        /> : <PhoneConfirmationDialog
          supabase={supabase}
          phoneNumber={phoneNumber}
          close={close}
          setError={setError}
          onSuccess={onSuccess}
        />}

        {loading && <div className="flex justify-center">
          <div className="animate-caret">
            <span className="sr-only">Loading...</span>
            <svg className="h-6 w-6 animate-spin fill-primary-foreground" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
          </div>
        </div>}
      </DialogContent>
    </Dialog >
  )
}

function InitialLoginDialog({
  supabase,
  handleFormSubmit,
  phoneNumber,
  setPhoneNumber,
  close,
  onSuccess,
}: {
  supabase: SupabaseClient,
  handleFormSubmit: (e: React.FormEvent<HTMLFormElement>) => void,
  phoneNumber: any,
  setPhoneNumber: (phoneNumber: any) => void,
  close: () => void,
  onSuccess?: () => void
}) {
  return (<>
    <form onSubmit={handleFormSubmit}>
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
          disabled={!isValidPhoneNumber(phoneNumber || "", "US")}
          type="submit"
          className="w-full"
        >Continue</Button>
      </div>
    </form >

    <div className="relative">
      <div className="absolute inset-0 flex items-center">
        <span className="w-full border-t" />
      </div>
      <div className="relative flex justify-center text-xs uppercase">
        <span className="bg-background px-2 text-muted-foreground">
          Or continue with
        </span>
      </div>
    </div>

    <div className="block flex justify-center">
      <GoogleLogin
        width={"100%"}
        onSuccess={credentialResponse => {
          if (credentialResponse.credential) {
            supabase.auth.signInWithIdToken({
              provider: 'google',
              token: credentialResponse.credential,
            })
            onSuccess && onSuccess()
            close()
          } else {
            alert("Failed to login with Google")
          }
        }}
        onError={() => {
          alert("Failed to login with Google")
        }}
      />
    </div>
  </>)
}

function PhoneConfirmationDialog({
  supabase,
  phoneNumber,
  close,
  setError,
  onSuccess,
}: {
  supabase: SupabaseClient,
  phoneNumber: string,
  close: () => void,
  setError: React.Dispatch<React.SetStateAction<string>>,
  onSuccess?: () => void
}) {
  const [confirmationCode, setConfirmationCode] = useState<string>("");

  const onSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();

    const {
      data,
      error,
    } = await supabase.auth.verifyOtp({
      phone: phoneNumber,
      token: confirmationCode,
      type: "sms",
    })
    if (error) {
      setError(error.message)
    } else {
      onSuccess && onSuccess()
      close()
    }
  }

  return (<>
    <form onSubmit={onSubmit}>
      <div className="grid gap-4">

        <div className="grid gap-2">
          <Label>SMS Confirmation Code</Label>
          <Input
            autoFocus
            // placeholder="+1 (408) 555-1234"
            // international
            required
            type="number"
            value={confirmationCode}
            onChange={(e) => {
              setError("")
              setConfirmationCode(e.target.value)
            }}
          />
        </div>
        <Button
          disabled={confirmationCode.toString().length !== 6}
          type="submit"
          className="w-full"
        >Continue</Button>
      </div>
    </form>
  </>)
}