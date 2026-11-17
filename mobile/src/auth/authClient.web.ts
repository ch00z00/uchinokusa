// ブラウザ（Expo の web）では @react-native-firebase が動かないので、Firebase の JS SDK を使う。
// サインインの状態は、ブラウザ（IndexedDB）に保存される
import { getAuth, signInWithPopup } from 'firebase/auth';

import { createFirebaseJsAuthClient, providerFor } from './firebaseJsAuth';

export const authClient = createFirebaseJsAuthClient({
  createAuth: getAuth,
  // Emulator では、Emulator のアカウント選択の画面がポップアップで開く
  signInWithProvider: (auth, provider) => signInWithPopup(auth, providerFor(provider)),
});
