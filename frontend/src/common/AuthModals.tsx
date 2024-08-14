import { useState } from "react";
import { useAuth } from "auth";
import modalsStyle from "./Modals.module.css";
import { GoogleLogin } from "@react-oauth/google";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@radix-ui/react-label";
import { Input } from "@/components/ui/input";
import { FaceIcon, ImageIcon, SunIcon } from '@radix-ui/react-icons';
import { Icon } from "lucide-react";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"

export function DialogDemo() {
  return (
    <Dialog open={true} onOpenChange={() => {}}>
      {/* <DialogTrigger asChild>
        <Button variant="outline">Edit Profile</Button>
      </DialogTrigger> */}
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>Edit profile</DialogTitle>
          <DialogDescription>
            Make changes to your profile here. Click save when you're done.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 py-4">
          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="name" className="text-right">
              Name
            </Label>
            <Input
              id="name"
              defaultValue="Pedro Duarte"
              className="col-span-3"
            />
          </div>
          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="username" className="text-right">
              Username
            </Label>
            <Input
              id="username"
              defaultValue="@peduarte"
              className="col-span-3"
            />
          </div>
        </div>
        <DialogFooter>
          <Button type="submit">Save changes</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function LoginModal({
  close,
  onSuccess,
}: {
  close: () => void,
  onSuccess?: () => void
}) {
  const { supabase, session } = useAuth()
  const [phone, setPhone] = useState<string>('');
  const [error, setError] = useState<string>('');

  if (!supabase) {
    return null;
  }

  const handleOverlayClick = (e: any) => {
    if (e.target.id === "login-modal") {
      close();
    }
  };

  if (session) {
    if (onSuccess) {
      onSuccess()
    }
    close();
  }

  return (
    <div id="login-modal" className={modalsStyle.modal} onClick={handleOverlayClick}>
      <div className={modalsStyle.modal_content}>

        <span onClick={() => close()} className={modalsStyle.close} id="closeModalBtn">&times;</span>
  {/* <CardsCreateAccount /> */}

        <h2>Login</h2>
        <div style={{ display: "flex", justifyContent: "center", marginTop: "40px" }}>


          <GoogleLogin
            onSuccess={credentialResponse => {
              if (credentialResponse.credential) {
                supabase.auth.signInWithIdToken({
                  provider: 'google',
                  token: credentialResponse.credential,
                })
                if (onSuccess) {
                  onSuccess()
                }

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
        {/* <p>or</p>
        <label>phone number</label>
        <input />

        <p>or</p>
        <label>phone number</label>
        <input /> */}
      </div>
    </div>
  );
}

export function CardsCreateAccount() {
  return (
    <Card>
      <CardHeader className="space-y-1">
        <CardTitle className="text-2xl">Create an account</CardTitle>
        <CardDescription>
          Enter your email below to create your account
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        <div className="grid grid-cols-2 gap-6">
          <Button variant="outline">
            {/* <Icon. className="mr-2 h-4 w-4" /> */}
            Github
          </Button>
          <Button variant="outline">
            {/* <Icons.google className="mr-2 h-4 w-4" /> */}
            Google
          </Button>
        </div>
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
        <div className="grid gap-2">
          <Label htmlFor="email">Email</Label>
          <Input id="email" type="email" placeholder="m@example.com" />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="password">Password</Label>
          <Input id="password" type="password" />
        </div>
      </CardContent>
      <CardFooter>
        <Button className="w-full">Create account</Button>
      </CardFooter>
    </Card>
  )
}