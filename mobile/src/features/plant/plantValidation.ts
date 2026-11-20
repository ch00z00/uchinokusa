import type { components } from '@/api/schema';

import type { Plant } from './plantList';

// 植物の入力の決まり（api/openapi.yaml の CreatePlantRequest。サーバーと同じ）：
// 名前は前後の空白を除いて1〜40文字、自由入力の種類は50文字まで（コードポイントで数える）
export const maxPlantNameLength = 40;
export const maxSpeciesFreeTextLength = 50;

/** 種類：植物マスタから選んだもの、自由入力、未設定のどれか（API はどちらか一方だけを持つ） */
export type SpeciesValue =
  { kind: 'none' } | { kind: 'species'; id: string; name: string } | { kind: 'free'; text: string };

export type PlantFormInput = {
  name: string;
  species: SpeciesValue;
  /** 空文字は未設定 */
  locationId: string;
  /** 購入日・お迎え日（`YYYY-MM-DD`）。空文字は未設定 */
  acquiredOn: string;
};

export const emptyPlantForm: PlantFormInput = { name: '', species: { kind: 'none' }, locationId: '', acquiredOn: '' };

export type PlantFieldError = 'required' | 'too_long' | 'future';

export type PlantFormResult =
  { ok: true; value: PlantFormInput } | { ok: false; errors: Partial<Record<keyof PlantFormInput, PlantFieldError>> };

const length = (s: string) => [...s].length;

/**
 * 入力を確かめる。today は端末での今日（`YYYY-MM-DD`）。
 * 購入日の未来の日は、サーバーはまだ受け付けるが（docs/backlog.md）、植え替えの提案の起点になるのでアプリで止める
 */
export function validatePlantForm(input: PlantFormInput, today: string): PlantFormResult {
  // trim は全角の空白（U+3000）も除く
  const name = input.name.trim();
  let species = input.species;
  if (species.kind === 'free') {
    const text = species.text.trim();
    species = text === '' ? { kind: 'none' } : { kind: 'free', text };
  }
  const errors: Partial<Record<keyof PlantFormInput, PlantFieldError>> = {};
  if (name === '') errors.name = 'required';
  else if (length(name) > maxPlantNameLength) errors.name = 'too_long';
  if (species.kind === 'free' && length(species.text) > maxSpeciesFreeTextLength) errors.species = 'too_long';
  // YYYY-MM-DD は文字列の順が日付の順
  if (input.acquiredOn !== '' && input.acquiredOn > today) errors.acquiredOn = 'future';
  if (Object.keys(errors).length > 0) return { ok: false, errors };
  return { ok: true, value: { ...input, name, species } };
}

/**
 * 送る値にする（作成と編集で同じ）。空文字は、作成では未設定、編集では未設定に戻す意味になる。
 * 種類は、どちらか一方を送ると、まるごと置き換わる（送らなかった方は消える）
 */
export function toPlantRequest(value: PlantFormInput): components['schemas']['CreatePlantRequest'] {
  const species =
    value.species.kind === 'species'
      ? { speciesId: value.species.id }
      : value.species.kind === 'free'
        ? { speciesFreeText: value.species.text }
        : { speciesId: '' };
  return { name: value.name, ...species, locationId: value.locationId, acquiredOn: value.acquiredOn };
}

/** API の植物を、入力欄の値にする（編集の初期値） */
export function plantToForm(
  plant: Pick<Plant, 'name' | 'speciesId' | 'speciesName' | 'speciesFreeText' | 'locationId' | 'acquiredOn'>,
): PlantFormInput {
  const species: SpeciesValue = plant.speciesId
    ? { kind: 'species', id: plant.speciesId, name: plant.speciesName ?? '' }
    : plant.speciesFreeText
      ? { kind: 'free', text: plant.speciesFreeText }
      : { kind: 'none' };
  return { name: plant.name, species, locationId: plant.locationId ?? '', acquiredOn: plant.acquiredOn ?? '' };
}
