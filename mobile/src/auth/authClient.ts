// iOS / Android の認証。
// いまは Firebase の JS SDK と Auth Emulator を使う。サインインの状態は AsyncStorage に保存し、再起動後も保つ。
// タスク 14.4 で、スマホ専用の @react-native-firebase/auth と、Apple / Google の OS のサインイン画面に置き換える（リリースの前提）。
import AsyncStorage from '@react-native-async-storage/async-storage';
// getReactNativePersistence の型は ./firebase-auth-rn.d.ts で補う
import { getReactNativePersistence, initializeAuth, signInWithCredential } from 'firebase/auth';

import { config } from '@/config';

import { createFirebaseJsAuthClient, emulatorCredential } from './firebaseJsAuth';

export const authClient = createFirebaseJsAuthClient({
  createAuth: (app) => initializeAuth(app, { persistence: getReactNativePersistence(AsyncStorage) }),
  signInWithProvider: async (auth, provider) => {
    if (!config.firebase.authEmulatorHost) {
      throw new Error('iOS / Android の本物のサインインは、タスク 14.4 で作る');
    }
    // JS SDK はスマホでポップアップを開けないので、Emulator の偽のトークンでサインインする
    return signInWithCredential(auth, emulatorCredential(provider));
  },
});
