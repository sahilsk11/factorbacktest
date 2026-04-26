import { createContext, useContext } from 'react';

export interface AppUser {
  id: string;
  email?: string | null;
  phoneNumber?: string | null;
  name?: string | null;
  image?: string | null;
}

export interface SessionResponse {
  user: AppUser | null;
}

export type AuthStatus = 'loading' | 'authenticated' | 'anonymous';

export interface SignInApi {
  google: () => Promise<void>;
  sendEmailCode: (email: string) => Promise<void>;
  verifyEmailCode: (email: string, code: string) => Promise<void>;
  sendSmsCode: (phoneNumber: string) => Promise<void>;
  verifySmsCode: (phoneNumber: string, code: string) => Promise<void>;
}

export interface AuthContextValue {
  status: AuthStatus;
  user: AppUser | null;
  isAuthenticated: boolean;
  signIn: SignInApi;
  signOut: () => Promise<void>;
  refreshSession: () => Promise<void>;
}

export const authSessionQueryKey = ['auth', 'session'] as const;

export const AuthContext = createContext<AuthContextValue | null>(null);

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
