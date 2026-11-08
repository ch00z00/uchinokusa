import { FirebaseError, initializeApp } from 'firebase/app';
import {
  connectAuthEmulator,
  createUserWithEmailAndPassword,
  getAuth,
  onAuthStateChanged,
  signInWithEmailAndPassword,
  signOut,
  updateProfile,
} from 'firebase/auth';

import { config } from '@/config';

import type { AuthClient } from './types';

// Auth Emulator のテストユーザー（server/README.md の例と同じ）
const devUser = { email: 'dev@example.com', password: 'password', displayName: '開発者' };

/** Firebase の JS SDK で認証する（ブラウザでの開発用） */
export function createFirebaseJsAuthClient(): AuthClient {
  const app = initializeApp({
    apiKey: config.firebase.apiKey,
    projectId: config.firebase.projectId,
    authDomain: `${config.firebase.projectId}.firebaseapp.com`,
  });
  const auth = getAuth(app);
  const emulatorHost = config.firebase.authEmulatorHost;
  if (emulatorHost) {
    connectAuthEmulator(auth, `http://${emulatorHost}`, { disableWarnings: true });
  }

  return {
    onAuthStateChanged: (listener) =>
      onAuthStateChanged(auth, (user) => listener(user ? { uid: user.uid, displayName: user.displayName } : null)),
    getIdToken: async (forceRefresh) => (auth.currentUser ? auth.currentUser.getIdToken(forceRefresh) : null),
    signOut: () => signOut(auth),
    signInForDevelopment: emulatorHost
      ? async () => {
          try {
            await signInWithEmailAndPassword(auth, devUser.email, devUser.password);
          } catch (error) {
            const notFound =
              error instanceof FirebaseError &&
              (error.code === 'auth/user-not-found' || error.code === 'auth/invalid-credential');
            if (!notFound) throw error;
            // Emulator のデータは再起動で消えるので、いなければ作る
            const { user } = await createUserWithEmailAndPassword(auth, devUser.email, devUser.password);
            await updateProfile(user, { displayName: devUser.displayName });
          }
        }
      : null,
  };
}
