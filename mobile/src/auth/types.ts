export type AuthUser = {
  uid: string;
  displayName: string | null;
};

export type AuthProviderId = 'apple' | 'google';

export type SignInResult =
  /** isNewUser：このプロバイダで初めてサインインした（表示名の設定を出す） */
  | { status: 'signedIn'; isNewUser: boolean }
  /** 利用者がサインインの画面を閉じた（何も表示しない） */
  | { status: 'cancelled' };

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
  signIn(provider: AuthProviderId): Promise<SignInResult>;
  signOut(): Promise<void>;
}
