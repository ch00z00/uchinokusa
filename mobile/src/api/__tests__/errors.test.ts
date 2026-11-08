import { ApiError, toApiError, toNetworkError } from '../errors';

describe('toApiError', () => {
  it('サーバーの code・message・fields を取り出す', () => {
    const error = toApiError(422, {
      code: 'validation_failed',
      message: 'invalid input',
      fields: { name: 'required' },
    });

    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(422);
    expect(error.code).toBe('validation_failed');
    expect(error.message).toBe('invalid input');
    expect(error.fields).toEqual({ name: 'required' });
  });

  it('衝突の内容などの追加の項目を body に残す', () => {
    const body = { code: 'already_completed', message: 'done', existing: { recordId: 'r1' } };

    expect(toApiError(409, body).body).toEqual(body);
  });

  it('知らない code は unknown にする（サーバーが code を足したとき）', () => {
    expect(toApiError(409, { code: 'new_conflict', message: 'x' }).code).toBe('unknown');
  });

  it('JSON でない本文は unknown にする（プロキシのエラーのページなど）', () => {
    const error = toApiError(502, undefined);

    expect(error.code).toBe('unknown');
    expect(error.status).toBe(502);
  });

  it('fields のうち、文字列でない値は捨てる', () => {
    expect(
      toApiError(422, { code: 'validation_failed', message: 'x', fields: { a: 'required', b: 1 } }).fields,
    ).toEqual({ a: 'required' });
  });
});

describe('toNetworkError', () => {
  it('status を null、code を network にする', () => {
    const cause = new TypeError('Network request failed');
    const error = toNetworkError(cause);

    expect(error.status).toBeNull();
    expect(error.code).toBe('network');
    expect(error.cause).toBe(cause);
  });
});
