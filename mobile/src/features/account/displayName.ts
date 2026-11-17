// 表示名の決まり（docs/decisions/0005、サーバーの domain.NormalizeDisplayName と同じ）：
// 前後の空白を除いて、1〜30文字。文字数はコードポイントで数える（Go の utf8.RuneCountInString と同じ）
export const maxDisplayNameLength = 30;

export type DisplayNameError = 'required' | 'too_long';

export type DisplayNameResult = { ok: true; value: string } | { ok: false; error: DisplayNameError };

export function validateDisplayName(input: string): DisplayNameResult {
  // trim は全角の空白（U+3000）も除く
  const value = input.trim();
  if (value === '') return { ok: false, error: 'required' };
  if ([...value].length > maxDisplayNameLength) return { ok: false, error: 'too_long' };
  return { ok: true, value };
}
