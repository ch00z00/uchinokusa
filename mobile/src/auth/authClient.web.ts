// ブラウザ（Expo の web）では @react-native-firebase が動かないので、Firebase の JS SDK を使う
import { createFirebaseJsAuthClient } from './firebaseJsAuth';

export const authClient = createFirebaseJsAuthClient();
