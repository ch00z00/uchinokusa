import { createContext, use, useEffect, useState, type ReactNode } from 'react';

import { authClient } from './authClient';
import type { AuthProviderId, AuthUser, SignInResult } from './types';

export type AuthState = { status: 'loading' } | { status: 'signedOut' } | { status: 'signedIn'; user: AuthUser };

type AuthContextValue = {
  state: AuthState;
  /** 初めてサインインしたので、表示名の確認（AUTH-02）が要る */
  needsProfileSetup: boolean;
  signIn: (provider: AuthProviderId) => Promise<SignInResult>;
  /** 表示名を確認・保存した */
  completeProfileSetup: () => void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' });
  const [needsProfileSetup, setNeedsProfileSetup] = useState(false);

  useEffect(
    () =>
      authClient.onAuthStateChanged((user) => {
        setState(user ? { status: 'signedIn', user } : { status: 'signedOut' });
        if (!user) setNeedsProfileSetup(false);
      }),
    [],
  );

  const signIn = async (provider: AuthProviderId) => {
    const result = await authClient.signIn(provider);
    if (result.status === 'signedIn' && result.isNewUser) setNeedsProfileSetup(true);
    return result;
  };

  return (
    <AuthContext value={{ state, needsProfileSetup, signIn, completeProfileSetup: () => setNeedsProfileSetup(false) }}>
      {children}
    </AuthContext>
  );
}

function useAuthContext(): AuthContextValue {
  const value = use(AuthContext);
  if (!value) throw new Error('AuthProvider の中で使う');
  return value;
}

export function useAuth(): AuthState {
  return useAuthContext().state;
}

export function useSignIn() {
  return useAuthContext().signIn;
}

export function useProfileSetup() {
  const { needsProfileSetup, completeProfileSetup } = useAuthContext();
  return { needsProfileSetup, completeProfileSetup };
}
