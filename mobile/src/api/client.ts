import createClient from 'openapi-fetch';

import { toApiError, toNetworkError } from './errors';
import type { paths } from './schema';

type FetchFn = (request: Request) => Promise<Response>;

export type AuthFetchOptions = {
  /** ID トークンを返す。forceRefresh が true なら、期限の前でも更新する。サインインしていなければ null */
  getIdToken: (forceRefresh: boolean) => Promise<string | null>;
  /** 更新したトークンでも 401 が返ったときに呼ぶ（サインイン画面へ戻す） */
  onUnauthenticated: () => void;
  fetch?: FetchFn;
};

function withToken(request: Request, token: string | null): Request {
  if (token === null) return request;
  const headers = new Headers(request.headers);
  headers.set('Authorization', `Bearer ${token}`);
  return new Request(request, { headers });
}

/**
 * ID トークンを付けて送る fetch を作る。
 * 401 が返ったら、トークンを更新して1回だけ送り直す（期限切れのトークンを送ってしまった場合のため）。
 */
export function createAuthFetch({
  getIdToken,
  onUnauthenticated,
  fetch = globalThis.fetch,
}: AuthFetchOptions): FetchFn {
  // 同時に複数のリクエストが 401 になっても、トークンの更新は1回にまとめる
  let refreshing: Promise<string | null> | null = null;
  const refreshToken = () => {
    refreshing ??= getIdToken(true).finally(() => {
      refreshing = null;
    });
    return refreshing;
  };

  return async (request) => {
    // 本文は1回しか読めないので、送り直す用に先に複製しておく
    const retryRequest = request.clone();
    const response = await fetch(withToken(request, await getIdToken(false)));
    if (response.status !== 401) return response;

    const token = await refreshToken();
    if (token === null) {
      onUnauthenticated();
      return response;
    }
    const retried = await fetch(withToken(retryRequest, token));
    if (retried.status === 401) onUnauthenticated();
    return retried;
  };
}

export type ApiClient = ReturnType<typeof createClient<paths>>;

export function createApiClient(baseUrl: string, fetch: FetchFn): ApiClient {
  return createClient<paths>({ baseUrl, fetch });
}

type ClientResult<D> = { data?: D; error?: unknown; response: Response };

/**
 * openapi-fetch の結果から data を取り出す。失敗は ApiError にして投げる（TanStack Query に失敗として渡すため）。
 *
 * @example
 * const me = await unwrap(api.GET('/me'));
 */
export async function unwrap<D>(request: Promise<ClientResult<D>>): Promise<D> {
  let result: ClientResult<D>;
  try {
    result = await request;
  } catch (cause) {
    throw toNetworkError(cause);
  }
  if (!result.response.ok) {
    throw toApiError(result.response.status, result.error);
  }
  // 204（本文なし）のときは undefined になる。その操作の D も undefined を含む型になっている
  return result.data as D;
}
