import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { endpoint } from "./config";

// Talks directly to the new Go auth package's /auth/* endpoints. No SDK,
// no JWT, no token refresh logic — the browser holds an HttpOnly session
// cookie that auto-attaches on cross-origin fetches that opt in via
// `credentials: "include"`.
//
// `AppSession.access_token` is kept on the type for backward-compat with
// existing call sites that send `Authorization: Bearer ${access_token}`.
// The header is now ignored by the backend (cookie middleware sets
// userAccountID first; the legacy JWT path is short-circuited). We leave
// the field as an empty string rather than removing it so we don't have
// to refactor every fetch in the codebase in this PR.

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
  // Email OTP is not implemented in the backend. Stubs preserved so call
  // sites compile; calling them throws.
  sendEmailOtp: (email: string) => Promise<void>;
  verifyEmailOtp: (email: string, otp: string) => Promise<void>;
  sendSmsOtp: (phoneNumber: string) => Promise<void>;
  verifySmsOtp: (phoneNumber: string, code: string) => Promise<void>;
}

interface AuthContextValue {
  loading: boolean;
  user: AppUser | null;
  session: AppSession | null;
  // Always false now. Kept on the type so consumers that branch on it
  // still compile; the JWT-fetch failure mode it represented can't
  // happen with cookie-only auth.
  tokenError: boolean;
  signIn: SignInApi;
  signOut: () => Promise<void>;
  refreshToken: () => Promise<string | null>;
}

const defaultSignIn: SignInApi = {
  google: async () => {},
  sendEmailOtp: async () => {
    throw new Error("email OTP is not enabled");
  },
  verifyEmailOtp: async () => {
    throw new Error("email OTP is not enabled");
  },
  sendSmsOtp: async () => {},
  verifySmsOtp: async () => {},
};

const AuthContext = createContext<AuthContextValue>({
  loading: true,
  user: null,
  session: null,
  tokenError: false,
  signIn: defaultSignIn,
  signOut: async () => {},
  refreshToken: async () => null,
});

const COMMON: RequestInit = { credentials: "include" };

interface AuthProviderProps {
  children: React.ReactNode;
}

const AuthProvider = ({ children }: AuthProviderProps) => {
  const [user, setUser] = useState<AppUser | null>(null);
  const [loading, setLoading] = useState<boolean>(true);

  const refresh = useCallback(async () => {
    try {
      const r = await fetch(`${endpoint}/auth/session`, COMMON);
      if (!r.ok) {
        setUser(null);
        return;
      }
      const body = (await r.json()) as { user: AppUser | null };
      setUser(body.user);
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const signOut = useCallback(async () => {
    await fetch(`${endpoint}/auth/sign-out`, {
      ...COMMON,
      method: "POST",
      headers: { "Content-Type": "application/json" },
    });
    await refresh();
  }, [refresh]);

  const signIn: SignInApi = useMemo(
    () => ({
      google: async () => {
        // Top-level redirect (not a fetch) so the browser follows Google's
        // OAuth flow naturally and lands the session cookie when it returns.
        window.location.assign(`${endpoint}/auth/google/start`);
      },
      sendEmailOtp: async () => {
        throw new Error("email OTP is not enabled");
      },
      verifyEmailOtp: async () => {
        throw new Error("email OTP is not enabled");
      },
      sendSmsOtp: async (phoneNumber) => {
        const r = await fetch(`${endpoint}/auth/sms/send`, {
          ...COMMON,
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ phoneNumber }),
        });
        // /auth/sms/send always 204 — uniform response, no enumeration leak.
        // Origin allowlist mismatch (403) is the only meaningful failure.
        if (r.status === 403) throw new Error("origin not allowed");
        if (!r.ok && r.status !== 204) throw new Error("failed to send SMS OTP");
      },
      verifySmsOtp: async (phoneNumber, code) => {
        const r = await fetch(`${endpoint}/auth/sms/verify`, {
          ...COMMON,
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ phoneNumber, code }),
        });
        if (r.status === 401) throw new Error("invalid SMS OTP");
        if (!r.ok && r.status !== 204) throw new Error("failed to verify SMS OTP");
        await refresh();
      },
    }),
    [refresh],
  );

  const session: AppSession | null = user ? { access_token: "", user } : null;

  const value: AuthContextValue = {
    loading,
    user,
    session,
    tokenError: false,
    signIn,
    signOut,
    refreshToken: async () => "",
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

export const useAuth = () => useContext(AuthContext);

export default AuthProvider;
