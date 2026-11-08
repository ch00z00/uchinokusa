import { createContext, use, useEffect, useState, type ReactNode } from 'react';

import { authClient } from './authClient';
import type { AuthUser } from './types';

export type AuthState = { status: 'loading' } | { status: 'signedOut' } | { status: 'signedIn'; user: AuthUser };

const AuthContext = createContext<AuthState>({ status: 'loading' });

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' });

  useEffect(
    () =>
      authClient.onAuthStateChanged((user) => setState(user ? { status: 'signedIn', user } : { status: 'signedOut' })),
    [],
  );

  return <AuthContext value={state}>{children}</AuthContext>;
}

export function useAuth(): AuthState {
  return use(AuthContext);
}
