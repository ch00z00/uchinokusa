import { QueryClient } from '@tanstack/react-query';

import { ApiError } from '@/api/errors';

const maxRetries = 3;

/**
 * 取得（GET）を自動で再試行するか。
 * 通信できないときとサーバーの障害（5xx）だけ再試行する。4xx は何度送っても同じ結果になるので再試行しない。
 * 待ち時間は TanStack Query の既定（1秒・2秒・4秒…の指数バックオフ）。
 */
export function shouldRetryQuery(failureCount: number, error: unknown): boolean {
  if (failureCount >= maxRetries) return false;
  if (!(error instanceof ApiError)) return false;
  return error.code === 'network' || (error.status !== null && error.status >= 500);
}

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: shouldRetryQuery },
      // 記録などの変更は、二重に送らないように自動では再試行しない（失敗を画面で知らせる）
      mutations: { retry: false },
    },
  });
}
