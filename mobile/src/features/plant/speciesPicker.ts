import { router } from 'expo-router';
import { Keyboard } from 'react-native';

import type { SpeciesValue } from './plantValidation';

// 種類の検索（PLANT-03 の /species のシート）と、開いた種類の欄をつなぐ（lib/choicePicker.ts と同じ形）
let request: { value: SpeciesValue; onChange: (value: SpeciesValue) => void } | null = null;

export function openSpeciesPicker(value: SpeciesValue, onChange: (value: SpeciesValue) => void): void {
  Keyboard.dismiss();
  request = { value, onChange };
  router.push('/species');
}

export function currentSpeciesRequest() {
  return request;
}

export function pickSpecies(value: SpeciesValue): void {
  request?.onChange(value);
  request = null;
}
