import { FirebaseError, initializeApp, type FirebaseApp } from 'firebase/app';
import {
  connectAuthEmulator,
  getAdditionalUserInfo,
  GoogleAuthProvider,
  onAuthStateChanged,
  OAuthProvider,
  signOut,
  type Auth,
  type AuthCredential,
  type AuthProvider,
  type UserCredential,
} from 'firebase/auth';

import { config } from '@/config';

import { AppleReauthUnavailable, type AuthClient, type AuthProviderId, type SignInResult } from './types';

/** プロバイダでのサインインのしかた（ブラウザはポップアップ、スマホは Emulator の偽のトークン） */
export type ProviderSignIn = (auth: Auth, provider: AuthProviderId) => Promise<UserCredential>;

// 利用者がサインインの画面を閉じたときのエラー
const cancelledCodes = new Set(['auth/popup-closed-by-user', 'auth/cancelled-popup-request']);

export function providerFor(provider: AuthProviderId): AuthProvider {
  return provider === 'apple' ? new OAuthProvider('apple.com') : new GoogleAuthProvider();
}

/**
 * Auth Emulator の偽のトークン。Emulator は署名を確かめないので、JSON の文字列をトークンとして受け付ける。
 * プロバイダごとに、決まった開発用のユーザーになる
 */
export function emulatorCredential(provider: AuthProviderId): AuthCredential {
  const idToken = JSON.stringify(
    provider === 'apple'
      ? // Apple は ID トークンに名前を含めない（表示名の入力を試せる）
        { sub: 'dev-apple', email: 'dev-apple@example.com', email_verified: true }
      : { sub: 'dev-google', email: 'dev-google@example.com', email_verified: true, name: '開発者（Google）' },
  );
  return provider === 'apple'
    ? new OAuthProvider('apple.com').credential({ idToken })
    : GoogleAuthProvider.credential(idToken);
}

/** Firebase の JS SDK で認証する（ブラウザと、14.4 までの iOS / Android） */
export function createFirebaseJsAuthClient(params: {
  createAuth: (app: FirebaseApp) => Auth;
  signInWithProvider: ProviderSignIn;
}): AuthClient {
  const app = initializeApp({
    apiKey: config.firebase.apiKey,
    projectId: config.firebase.projectId,
    authDomain: `${config.firebase.projectId}.firebaseapp.com`,
  });
  const auth = params.createAuth(app);
  if (config.firebase.authEmulatorHost) {
    connectAuthEmulator(auth, `http://${config.firebase.authEmulatorHost}`, { disableWarnings: true });
  }

  return {
    onAuthStateChanged: (listener) =>
      onAuthStateChanged(auth, (user) => listener(user ? { uid: user.uid, displayName: user.displayName } : null)),
    getIdToken: async (forceRefresh) => (auth.currentUser ? auth.currentUser.getIdToken(forceRefresh) : null),
    signIn: async (provider): Promise<SignInResult> => {
      try {
        const credential = await params.signInWithProvider(auth, provider);
        return { status: 'signedIn', isNewUser: getAdditionalUserInfo(credential)?.isNewUser ?? false };
      } catch (error) {
        if (error instanceof FirebaseError && cancelledCodes.has(error.code)) return { status: 'cancelled' };
        throw error;
      }
    },
    signOut: () => signOut(auth),
    currentProvider: () => {
      const providerId = auth.currentUser?.providerData[0]?.providerId;
      if (providerId === 'apple.com') return 'apple';
      if (providerId === 'google.com') return 'google';
      return null;
    },
    // JS SDK では Apple の認可コードを受け取れない。14.4 で、OS の Apple のサインイン（expo-apple-authentication）に置き換える
    appleAuthorizationCode: () => Promise.reject(new AppleReauthUnavailable()),
  };
}
