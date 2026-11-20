import type { components } from '@/api/schema';

// 置き場所の入力の決まり（api/openapi.yaml の CreateLocationRequest。サーバーと同じ）：
// 置き場所名は前後の空白を除いて1〜30文字（コードポイントで数える）。環境の項目は任意で、空文字は未設定
export const maxLocationNameLength = 30;

export type Location = components['schemas']['Location'];
export type WindowDirection = components['schemas']['WindowDirection'];
export type WindowDistance = components['schemas']['WindowDistance'];
export type Shading = components['schemas']['Shading'];

// 選択肢の並び（シートに出す順）
export const windowDirections = [
  'n',
  'ne',
  'e',
  'se',
  's',
  'sw',
  'w',
  'nw',
] as const satisfies readonly WindowDirection[];
export const windowDistances = ['near', 'middle', 'far'] as const satisfies readonly WindowDistance[];
export const shadings = ['none', 'sheer'] as const satisfies readonly Shading[];

export type LocationFormInput = {
  name: string;
  isIndoor: boolean;
  /** 空文字は未設定 */
  windowDirection: WindowDirection | '';
  windowDistance: WindowDistance | '';
  shading: Shading | '';
};

export const emptyLocationForm: LocationFormInput = {
  name: '',
  isIndoor: true,
  windowDirection: '',
  windowDistance: '',
  shading: '',
};

export type LocationFieldError = 'required' | 'too_long';

export type LocationFormResult =
  | { ok: true; value: LocationFormInput }
  | { ok: false; errors: Partial<Record<keyof LocationFormInput, LocationFieldError>> };

type EnvironmentFields = Pick<Location, 'isIndoor' | 'windowDirection' | 'windowDistance' | 'shading'>;

// API の値はあとから増えうる。アプリが知らない値は未設定として扱う（openapi.yaml の WindowDirection などの説明）
const known = <T extends string>(values: readonly T[], value: string | undefined): T | '' =>
  values.includes(value as T) ? (value as T) : '';

/** API の置き場所を、入力欄の値にする（編集の初期値） */
export function locationToForm(location: Pick<Location, 'name'> & EnvironmentFields): LocationFormInput {
  return {
    name: location.name,
    isIndoor: location.isIndoor,
    windowDirection: known(windowDirections, location.windowDirection),
    windowDistance: known(windowDistances, location.windowDistance),
    shading: known(shadings, location.shading),
  };
}

const length = (s: string) => [...s].length;

/**
 * 入力を確かめ、送る値にする。屋外では、窓からの距離と遮光は意味がないので未設定に戻す
 * （編集で屋外に変えたときに、前の値が残らないように空文字を送る）
 */
export function validateLocationForm(input: LocationFormInput): LocationFormResult {
  // trim は全角の空白（U+3000）も除く
  const name = input.name.trim();
  if (name === '') return { ok: false, errors: { name: 'required' } };
  if (length(name) > maxLocationNameLength) return { ok: false, errors: { name: 'too_long' } };
  return {
    ok: true,
    value: {
      ...input,
      name,
      windowDistance: input.isIndoor ? input.windowDistance : '',
      shading: input.isIndoor ? input.shading : '',
    },
  };
}

/** 一覧（LOC-01）に出す環境の要約の文言のキー（例：室内・南・窓際・レースカーテン）。未設定の項目は出さない */
export function locationSummaryKeys(location: EnvironmentFields) {
  const form = locationToForm({ name: '', ...location });
  const keys = [`location.indoor.${form.isIndoor ? 'true' : 'false'}` as const] as string[];
  if (form.windowDirection) keys.push(`location.windowDirection.${form.windowDirection}`);
  if (form.isIndoor && form.windowDistance) keys.push(`location.windowDistance.${form.windowDistance}`);
  if (form.isIndoor && form.shading) keys.push(`location.shading.${form.shading}`);
  return keys as (
    | `location.indoor.${'true' | 'false'}`
    | `location.windowDirection.${WindowDirection}`
    | `location.windowDistance.${WindowDistance}`
    | `location.shading.${Shading}`
  )[];
}
