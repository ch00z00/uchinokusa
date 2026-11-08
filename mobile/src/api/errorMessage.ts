import type { ApiErrorCode } from './errors';
import { ApiError } from './errors';

/** エラーを画面に出すときの文言のキー（ja.json の errors.<code>） */
export function errorMessageKey(error: unknown): `errors.${ApiErrorCode}` {
  return `errors.${error instanceof ApiError ? error.code : 'unknown'}`;
}
