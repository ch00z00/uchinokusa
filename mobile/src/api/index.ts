import { authClient } from '@/auth/authClient';
import { config } from '@/config';

import { createApiClient, createAuthFetch } from './client';

export { unwrap } from './client';
export { ApiError, type ApiErrorCode } from './errors';

/**
 * アプリで使う API のクライアント。パスと引数・戻り値の型は api/openapi.yaml から生成した型で決まる。
 *
 * @example
 * const me = await unwrap(api.GET('/me'));
 */
export const api = createApiClient(
  config.apiBaseUrl,
  createAuthFetch({
    getIdToken: (forceRefresh) => authClient.getIdToken(forceRefresh),
    // 更新したトークンでも認証できない：サインアウトすると、サインイン画面に戻る
    onUnauthenticated: () => void authClient.signOut(),
  }),
);
