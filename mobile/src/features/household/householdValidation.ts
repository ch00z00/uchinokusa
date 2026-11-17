// 世帯の入力の決まり（api/openapi.yaml の CreateHouseholdRequest。サーバーと同じ）：
// 世帯名は前後の空白を除いて1〜30文字、市区町村は50文字まで。文字数はコードポイントで数える
export const maxHouseholdNameLength = 30;
export const maxRegionLength = 50;

export type HouseholdFormInput = {
  name: string;
  /** 都道府県。一覧から選ぶ。空文字は未設定 */
  regionPrefecture: string;
  /** 市区町村。空文字は未設定 */
  regionCity: string;
};

export type HouseholdFieldError = 'required' | 'too_long';

export type HouseholdFormResult =
  | { ok: true; value: HouseholdFormInput }
  | { ok: false; errors: Partial<Record<keyof HouseholdFormInput, HouseholdFieldError>> };

const length = (s: string) => [...s].length;

export function validateHouseholdForm(input: HouseholdFormInput): HouseholdFormResult {
  // trim は全角の空白（U+3000）も除く
  const value = {
    name: input.name.trim(),
    regionPrefecture: input.regionPrefecture,
    regionCity: input.regionCity.trim(),
  };
  const errors: Partial<Record<keyof HouseholdFormInput, HouseholdFieldError>> = {};
  if (value.name === '') errors.name = 'required';
  else if (length(value.name) > maxHouseholdNameLength) errors.name = 'too_long';
  if (length(value.regionCity) > maxRegionLength) errors.regionCity = 'too_long';
  return Object.keys(errors).length === 0 ? { ok: true, value } : { ok: false, errors };
}
