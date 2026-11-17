import { maxDisplayNameLength, validateDisplayName } from '../displayName';

describe('validateDisplayName', () => {
  it('前後の空白（全角の空白を含む）を除いた値を返す', () => {
    expect(validateDisplayName('　 花子 \n')).toEqual({ ok: true, value: '花子' });
  });

  it('途中の空白は残す', () => {
    expect(validateDisplayName('山田 花子')).toEqual({ ok: true, value: '山田 花子' });
  });

  it.each(['', '   ', '　　'])('空（%j）なら required', (input) => {
    expect(validateDisplayName(input)).toEqual({ ok: false, error: 'required' });
  });

  it('30文字までは通し、31文字は too_long', () => {
    expect(validateDisplayName('あ'.repeat(maxDisplayNameLength))).toEqual({
      ok: true,
      value: 'あ'.repeat(maxDisplayNameLength),
    });
    expect(validateDisplayName('あ'.repeat(maxDisplayNameLength + 1))).toEqual({ ok: false, error: 'too_long' });
  });

  it('絵文字などのサロゲートペアは1文字と数える（サーバーのコードポイントの数え方と同じ）', () => {
    // '🌱' は UTF-16 では2単位（length が 2）だが、1文字
    expect(validateDisplayName('🌱'.repeat(maxDisplayNameLength)).ok).toBe(true);
  });
});
