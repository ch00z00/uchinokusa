// iOS / Android の認証。
// いまはブラウザと同じ Firebase の JS SDK を使う（サインインの状態は、アプリを再起動すると消える）。
// タスク 14.2 で、スマホ専用の @react-native-firebase/auth と Apple / Google のサインインに置き換える（リリースの前提）。
import { createFirebaseJsAuthClient } from './firebaseJsAuth';

export const authClient = createFirebaseJsAuthClient();
