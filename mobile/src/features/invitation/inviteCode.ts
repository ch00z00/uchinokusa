// 招待コードの決まり（api/openapi.yaml の Invitation.code。サーバーの domain.InvitationCodeAlphabet と同じ）：
// 読み間違えやすい 0・O・1・I・L を除いた英大文字と数字の8文字
const codePattern = /^[ABCDEFGHJKMNPQRSTUVWXYZ23456789]{8}$/;

export type InviteCodeError = 'required' | 'invalid';

export type InviteCodeResult = { ok: true; value: string } | { ok: false; error: InviteCodeError };

/**
 * 手で入力した招待コードをそろえて確かめる（INV-10）。
 * 口頭で伝えたときに入りがちな空白・ハイフンを除き、全角を半角に、小文字を大文字にする
 */
export function normalizeInviteCode(input: string): InviteCodeResult {
  const value = input
    .normalize('NFKC') // 全角の英数字・空白を半角に
    .replace(/[\s\-‐−ー]/g, '')
    .toUpperCase();
  if (value === '') return { ok: false, error: 'required' };
  if (!codePattern.test(value)) return { ok: false, error: 'invalid' };
  return { ok: true, value };
}
