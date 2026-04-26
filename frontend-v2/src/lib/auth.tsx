import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, type ReactNode } from 'react';

import { apiClient, apiUrl } from '@/lib/api';
import {
  AuthContext,
  authSessionQueryKey,
  type AuthContextValue,
  type AuthStatus,
  type SessionResponse,
  type SignInApi,
} from '@/lib/auth-context';

const getSession = () => apiClient.get<SessionResponse>('/auth/session');

export function AuthProvider({ children }: { children: ReactNode }): React.ReactNode {
  const queryClient = useQueryClient();
  const sessionQuery = useQuery({
    queryKey: authSessionQueryKey,
    queryFn: getSession,
    retry: false,
    staleTime: 30_000,
  });

  const refreshSession = useCallback(async () => {
    return queryClient.fetchQuery({
      queryKey: authSessionQueryKey,
      queryFn: getSession,
      staleTime: 0,
    });
  }, [queryClient]);

  const signIn = useMemo<SignInApi>(
    () => ({
      google: async () => {
        window.location.assign(apiUrl('/auth/google/start'));
      },
      sendEmailCode: (email) =>
        apiClient.post<undefined>('/auth/email/send', {
          email: email.trim().toLowerCase(),
        }),
      verifyEmailCode: async (email, code) => {
        await apiClient.post<undefined>('/auth/email/verify', {
          email: email.trim().toLowerCase(),
          code: code.trim(),
        });
        const session = await refreshSession();
        await queryClient.invalidateQueries({ predicate: isNonAuthQuery });
        return session.user;
      },
      sendSmsCode: (phoneNumber) =>
        apiClient.post<undefined>('/auth/sms/send', {
          phoneNumber: phoneNumber.trim(),
        }),
      verifySmsCode: async (phoneNumber, code) => {
        await apiClient.post<undefined>('/auth/sms/verify', {
          phoneNumber: phoneNumber.trim(),
          code: code.trim(),
        });
        const session = await refreshSession();
        await queryClient.invalidateQueries({ predicate: isNonAuthQuery });
        return session.user;
      },
    }),
    [queryClient, refreshSession],
  );

  const signOut = useCallback(async () => {
    let signOutError: unknown;

    try {
      await apiClient.post<undefined>('/auth/sign-out');
    } catch (err: unknown) {
      signOutError = err;
    } finally {
      queryClient.removeQueries({ predicate: isNonAuthQuery });
      queryClient.setQueryData<SessionResponse>(authSessionQueryKey, { user: null });

      try {
        await refreshSession();
      } catch (err: unknown) {
        signOutError ??= err;
      }
    }

    if (signOutError) throw signOutError;
  }, [queryClient, refreshSession]);

  const user = sessionQuery.data?.user ?? null;
  const status: AuthStatus = sessionQuery.isPending
    ? 'loading'
    : user
      ? 'authenticated'
      : 'anonymous';

  const value = useMemo<AuthContextValue>(
    () => ({
      status,
      user,
      isAuthenticated: status === 'authenticated',
      signIn,
      signOut,
      refreshSession,
    }),
    [refreshSession, signIn, signOut, status, user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

function isNonAuthQuery(query: { queryKey: readonly unknown[] }): boolean {
  return query.queryKey[0] !== 'auth';
}
