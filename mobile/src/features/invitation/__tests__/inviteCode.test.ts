import { normalizeInviteCode } from '../inviteCode';

describe('normalizeInviteCode', () => {
  it('8文字のコードを受け付ける', () => {
    expect(normalizeInviteCode('K7MP2QXA')).toEqual({ ok: true, value: 'K7MP2QXA' });
  });

  it('小文字で入れても、大文字にそろえる（大文字・小文字を区別しない）', () => {
    expect(normalizeInviteCode('k7mp2qxa')).toEqual({ ok: true, value: 'K7MP2QXA' });
  });

  it('口頭で伝えたときに入りがちな、空白とハイフンを除く', () => {
    expect(normalizeInviteCode(' K7MP-2QXA ')).toEqual({ ok: true, value: 'K7MP2QXA' });
    expect(normalizeInviteCode('K7MP　2QXA')).toEqual({ ok: true, value: 'K7MP2QXA' });
  });

  it('全角の英数字は半角にする', () => {
    expect(normalizeInviteCode('Ｋ７ＭＰ２ＱＸＡ')).toEqual({ ok: true, value: 'K7MP2QXA' });
  });

  it('空なら required', () => {
    expect(normalizeInviteCode('  ')).toEqual({ ok: false, error: 'required' });
  });

  it.each(['K7MP2QX', 'K7MP2QXAB'])('8文字でなければ invalid（%s）', (input) => {
    expect(normalizeInviteCode(input)).toEqual({ ok: false, error: 'invalid' });
  });

  it.each(['K7MP2QX0', 'K7MP2QXO', 'K7MP2QX1', 'K7MP2QXI', 'K7MP2QXL', 'K7MP2QX!'])(
    'コードに使わない文字があれば invalid（%s）',
    (input) => {
      expect(normalizeInviteCode(input)).toEqual({ ok: false, error: 'invalid' });
    },
  );
});
