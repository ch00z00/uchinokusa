export type AuthUser = {
  uid: string;
  displayName: string | null;
};

/**
 * 認証の取り決め。プラットフォームごとに実装を分ける。
 * - `authClient.web.ts`：ブラウザ（Expo の web）。Firebase の JS SDK
 * - `authClient.ts`：iOS / Android
 */
export interface AuthClient {
  /** サインインの状態が変わるたびに呼ばれる（登録した直後にも1回呼ばれる）。戻り値で登録を解除する */
  onAuthStateChanged(listener: (user: AuthUser | null) => void): () => void;
  /** ID トークンを返す。forceRefresh が true なら、期限の前でも更新する。サインインしていなければ null */
  getIdToken(forceRefresh: boolean): Promise<string | null>;
  signOut(): Promise<void>;
  /** 開発用：Auth Emulator のテストユーザーでサインインする。Emulator を使っていなければ null */
  signInForDevelopment: (() => Promise<void>) | null;
}
