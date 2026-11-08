import { ApiError } from '@/api/errors';

import { shouldRetryQuery } from '../queryClient';

const apiError = (status: number | null, code: ApiError['code']) => new ApiError({ status, code, message: '' });

describe('shouldRetryQuery', () => {
  it.each([
    ['通信できない', apiError(null, 'network')],
    ['サーバーの障害（500）', apiError(500, 'internal')],
    ['サーバーの障害（503、本文が JSON でない）', apiError(503, 'unknown')],
  ])('%s なら再試行する', (_, error) => {
    expect(shouldRetryQuery(0, error)).toBe(true);
  });

  it.each([
    ['権限がない（403）', apiError(403, 'forbidden')],
    ['見つからない（404）', apiError(404, 'not_found')],
    ['未認証（401。トークンの更新は通信の層で済ませている）', apiError(401, 'unauthenticated')],
    ['ApiError でないエラー（アプリの不具合）', new Error('bug')],
  ])('%s なら再試行しない', (_, error) => {
    expect(shouldRetryQuery(0, error)).toBe(false);
  });

  it('3回失敗したら、それ以上は再試行しない', () => {
    expect(shouldRetryQuery(2, apiError(null, 'network'))).toBe(true);
    expect(shouldRetryQuery(3, apiError(null, 'network'))).toBe(false);
  });
});
