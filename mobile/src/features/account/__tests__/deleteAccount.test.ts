import { ApiError } from '@/api/errors';

import { classifyDeleteError } from '../deleteAccount';

describe('classifyDeleteError', () => {
  it('唯一のオーナーの世帯があれば、その世帯を返す（要件1-7）', () => {
    const error = new ApiError({
      status: 409,
      code: 'owner_transfer_required',
      message: 'x',
      body: { code: 'owner_transfer_required', message: 'x', households: [{ id: 'h1', name: '俺の家' }] },
    });
    expect(classifyDeleteError(error)).toEqual({
      kind: 'owner_transfer_required',
      households: [{ id: 'h1', name: '俺の家' }],
    });
  });

  it('Apple の認可コードがない・無効なら、再認証し直す', () => {
    const error = new ApiError({
      status: 422,
      code: 'validation_failed',
      message: 'x',
      fields: { appleAuthorizationCode: 'invalid' },
    });
    expect(classifyDeleteError(error)).toEqual({ kind: 'apple_reauth' });
  });

  it('ほかは、ふつうのエラーとして出す', () => {
    const error = new ApiError({ status: null, code: 'network', message: 'x' });
    expect(classifyDeleteError(error)).toEqual({ kind: 'other', error });
  });
});
