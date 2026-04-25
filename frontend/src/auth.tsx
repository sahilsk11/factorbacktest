import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { createAuthClient } from "better-auth/react";
import { emailOTPClient, phoneNumberClient } from "better-auth/client/plugins";
import { endpoint } from "./config";

// The auth-service is mounted at /api/auth on the same domain as the Go API
// (same Fly machine). In local dev that means localhost:3009, in prod the
// Fly hostname. We reuse `endpoint` so this stays in sync with every other
// API call in the app.
const authClient = createAuthClient({
  baseURL: endpoint,
  plugins: [emailOTPClient(), phoneNumberClient()],
});

// Shape kept compatible with existing call sites (`session.access_token`,
// `session.user.id`, etc.) so the wider app didn't have to change. The
// access token is fetched on demand from Better Auth's JWT plugin endpoint.
export interface AppUser {
  id: string;
  email?: string | null;
  phoneNumber?: string | null;
  name?: string | null;
  image?: string | null;
}

export interface AppSession {
  access_token: string;
  user: AppUser;
}

interface SignInApi {
  google: () => Promise<void>;
  sendEmailOtp: (email: string) => Promise<void>;
  verifyEmailOtp: (email: string, otp: string) => Promise<void>;
  sendSmsOtp: (phoneNumber: string) => Promise<void>;
  verifySmsOtp: (phoneNumber: string, code: string) => Promise<void>;
}

interface AuthContextValue {
  loading: boolean;
  user: AppUser | null;
  session: AppSession | null;
  signIn: SignInApi;
  signOut: () => Promise<void>;
  refreshToken: () => Promise<string | null>;
}

const defaultSignIn: SignInApi = {
  google: async () => {},
  sendEmailOtp: async () => {},
  verifyEmailOtp: async () => {},
  sendSmsOtp: async () => {},
  verifySmsOtp: async () => {},
};

const AuthContext = createContext<AuthContextValue>({
  loading: true,
  user: null,
  session: null,
  signIn: defaultSignIn,
  signOut: async () => {},
  refreshToken: async () => null,
});

const fetchAccessToken = async (): Promise<string | null> => {
  try {
    const resp = await fetch(`${endpoint}/api/auth/token`, { credentials: "include" });
    if (!resp.ok) return null;
    const body = (await resp.json()) as { token?: string };
    return body.token ?? null;
  } catch {
    return null;
  }
};

interface AuthProviderProps {
  children: React.ReactNode;
}

const AuthProvider = ({ children }: AuthProviderProps) => {
  const { data, isPending, refetch } = authClient.useSession();
  const [accessToken, setAccessToken] = useState<string | null>(null);
  // Tracks the "I have a session but I'm still fetching the JWT" window so
  // page-level guards don't briefly see (loading=false, session=null) on
  // refresh and pop a login modal at an already-authenticated user.
  const [tokenLoading, setTokenLoading] = useState<boolean>(false);

  const sessionUser = data?.user as AppUser | undefined;

  const sessionUserId = sessionUser?.id ?? null;
  useEffect(() => {
    let cancelled = false;
    if (!sessionUserId) {
      setAccessToken(null);
      setTokenLoading(false);
      return;
    }
    setTokenLoading(true);
    fetchAccessToken().then((token) => {
      if (cancelled) return;
      setAccessToken(token);
      setTokenLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, [sessionUserId]);

  const refreshToken = useCallback(async () => {
    const token = await fetchAccessToken();
    setAccessToken(token);
    return token;
  }, []);

  const signOut = useCallback(async () => {
    await authClient.signOut();
    await refetch();
  }, [refetch]);

  const signIn: SignInApi = useMemo(
    () => ({
      google: async () => {
        await authClient.signIn.social({
          provider: "google",
          callbackURL: window.location.href,
        });
      },
      sendEmailOtp: async (email) => {
        const { error } = await authClient.emailOtp.sendVerificationOtp({
          email,
          type: "sign-in",
        });
        if (error) throw new Error(error.message ?? "failed to send email OTP");
      },
      verifyEmailOtp: async (email, otp) => {
        const { error } = await authClient.signIn.emailOtp({ email, otp });
        if (error) throw new Error(error.message ?? "invalid email OTP");
        await refetch();
      },
      sendSmsOtp: async (phoneNumber) => {
        const { error } = await authClient.phoneNumber.sendOtp({ phoneNumber });
        if (error) throw new Error(error.message ?? "failed to send SMS OTP");
      },
      verifySmsOtp: async (phoneNumber, code) => {
        const { error } = await authClient.phoneNumber.verify({ phoneNumber, code });
        if (error) throw new Error(error.message ?? "invalid SMS OTP");
        await refetch();
      },
    }),
    [refetch],
  );

  const session: AppSession | null =
    sessionUser && accessToken
      ? { access_token: accessToken, user: sessionUser }
      : null;

  const value: AuthContextValue = {
    loading: isPending || tokenLoading,
    user: sessionUser ?? null,
    session,
    signIn,
    signOut,
    refreshToken,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

export const useAuth = () => useContext(AuthContext);

export default AuthProvider;
