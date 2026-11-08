import ja from '@/i18n/ja.json';

import { errorMessageKey } from '../errorMessage';
import { ApiError, apiErrorCodes } from '../errors';

describe('errorMessageKey', () => {
  it('ApiError の code からキーを作る', () => {
    expect(errorMessageKey(new ApiError({ status: 403, code: 'forbidden', message: '' }))).toBe('errors.forbidden');
  });

  it('ApiError でないエラーは unknown にする', () => {
    expect(errorMessageKey(new Error('bug'))).toBe('errors.unknown');
  });

  it.each(apiErrorCodes)('ja.json に errors.%s の文言がある', (code) => {
    expect(ja.errors[code]).toEqual(expect.any(String));
  });
});
