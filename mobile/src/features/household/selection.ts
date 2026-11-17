/**
 * 表示する世帯を決める（要件2-4）。
 * 端末に保存した世帯に今も所属していればその世帯、そうでなければ最初の世帯（作成の古い順の先頭）。
 * 所属する世帯がなければ null（はじめての選択 ONB-01 を出す）
 */
export function resolveSelectedHouseholdId(
  households: readonly { id: string }[],
  storedId: string | null,
): string | null {
  if (storedId !== null && households.some((h) => h.id === storedId)) return storedId;
  return households[0]?.id ?? null;
}
