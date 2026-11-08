import { createApiClient, createAuthFetch, unwrap } from '../client';
import { ApiError } from '../errors';

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

/** 受け取ったリクエストを記録し、決めた順にレスポンスを返す fetch */
function fakeFetch(...responses: Response[]) {
  const requests: Request[] = [];
  const fetch = jest.fn(async (request: Request) => {
    requests.push(request);
    const response = responses.shift();
    if (!response) throw new Error('予定より多く呼ばれた');
    return response;
  });
  return { fetch, requests };
}

describe('createAuthFetch', () => {
  it('ID トークンを Authorization ヘッダーに付ける', async () => {
    const { fetch, requests } = fakeFetch(jsonResponse(200, {}));
    const authFetch = createAuthFetch({ getIdToken: async () => 'token-1', onUnauthenticated: jest.fn(), fetch });

    await authFetch(new Request('http://api.test/v1/me'));

    expect(requests[0].headers.get('Authorization')).toBe('Bearer token-1');
  });

  it('401 なら、トークンを更新して同じ本文で1回だけ送り直す', async () => {
    const { fetch, requests } = fakeFetch(jsonResponse(401, {}), jsonResponse(200, { ok: true }));
    const getIdToken = jest.fn(async (forceRefresh: boolean) => (forceRefresh ? 'fresh' : 'stale'));
    const onUnauthenticated = jest.fn();
    const authFetch = createAuthFetch({ getIdToken, onUnauthenticated, fetch });

    const response = await authFetch(
      new Request('http://api.test/v1/me', { method: 'PUT', body: JSON.stringify({ displayName: 'a' }) }),
    );

    expect(response.status).toBe(200);
    expect(requests).toHaveLength(2);
    expect(requests[1].headers.get('Authorization')).toBe('Bearer fresh');
    expect(await requests[1].text()).toBe('{"displayName":"a"}');
    expect(onUnauthenticated).not.toHaveBeenCalled();
  });

  it('送り直しても 401 なら、onUnauthenticated を呼んで 401 を返す', async () => {
    const { fetch } = fakeFetch(jsonResponse(401, {}), jsonResponse(401, {}));
    const onUnauthenticated = jest.fn();
    const authFetch = createAuthFetch({ getIdToken: async () => 't', onUnauthenticated, fetch });

    const response = await authFetch(new Request('http://api.test/v1/me'));

    expect(response.status).toBe(401);
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(onUnauthenticated).toHaveBeenCalledTimes(1);
  });

  it('サインインしていなければ、送り直さずに onUnauthenticated を呼ぶ', async () => {
    const { fetch, requests } = fakeFetch(jsonResponse(401, {}));
    const onUnauthenticated = jest.fn();
    const authFetch = createAuthFetch({ getIdToken: async () => null, onUnauthenticated, fetch });

    await authFetch(new Request('http://api.test/v1/me'));

    expect(requests[0].headers.has('Authorization')).toBe(false);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(onUnauthenticated).toHaveBeenCalledTimes(1);
  });

  it('同時に 401 になったリクエストでは、トークンの更新を1回にまとめる', async () => {
    const { fetch } = fakeFetch(
      jsonResponse(401, {}),
      jsonResponse(401, {}),
      jsonResponse(200, {}),
      jsonResponse(200, {}),
    );
    const getIdToken = jest.fn(async (_forceRefresh: boolean) => 't');
    const authFetch = createAuthFetch({ getIdToken, onUnauthenticated: jest.fn(), fetch });

    await Promise.all([authFetch(new Request('http://api.test/v1/a')), authFetch(new Request('http://api.test/v1/b'))]);

    expect(getIdToken.mock.calls.filter(([forceRefresh]) => forceRefresh)).toHaveLength(1);
  });
});

describe('unwrap', () => {
  it('成功なら data を返す', async () => {
    const { fetch } = fakeFetch(
      jsonResponse(200, { id: 'u1', displayName: '花子', createdAt: '2026-11-08T00:00:00Z' }),
    );
    const api = createApiClient('http://api.test/v1', fetch);

    await expect(unwrap(api.GET('/me'))).resolves.toEqual(expect.objectContaining({ displayName: '花子' }));
  });

  it('エラーのレスポンスは ApiError にして投げる', async () => {
    const { fetch } = fakeFetch(jsonResponse(404, { code: 'not_found', message: 'not found' }));
    const api = createApiClient('http://api.test/v1', fetch);

    await expect(unwrap(api.GET('/me'))).rejects.toMatchObject({ status: 404, code: 'not_found' });
  });

  it('サーバーに届かなければ、code が network の ApiError にして投げる', async () => {
    const api = createApiClient('http://api.test/v1', async () => {
      throw new TypeError('Network request failed');
    });

    const error = await unwrap(api.GET('/me')).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: null, code: 'network' });
  });
});
