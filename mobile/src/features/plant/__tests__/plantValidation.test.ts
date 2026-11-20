import {
  maxPlantNameLength,
  maxSpeciesFreeTextLength,
  plantToForm,
  toPlantRequest,
  validatePlantForm,
  type PlantFormInput,
} from '../plantValidation';

const base: PlantFormInput = { name: 'ポトス', species: { kind: 'none' }, locationId: '', acquiredOn: '' };
const today = '2026-11-20';

describe('validatePlantForm', () => {
  it('名前の前後の空白（全角も）を除く', () => {
    expect(validatePlantForm({ ...base, name: '　ポトス ' }, today)).toEqual({ ok: true, value: base });
  });

  it('名前が空なら required、40文字を超えたら too_long（コードポイントで数える）', () => {
    expect(validatePlantForm({ ...base, name: ' ' }, today)).toEqual({ ok: false, errors: { name: 'required' } });
    expect(validatePlantForm({ ...base, name: '🌱'.repeat(maxPlantNameLength) }, today).ok).toBe(true);
    expect(validatePlantForm({ ...base, name: '🌱'.repeat(maxPlantNameLength + 1) }, today)).toEqual({
      ok: false,
      errors: { name: 'too_long' },
    });
  });

  it('自由入力の種類は前後の空白を除き、空なら未設定、50文字を超えたら too_long', () => {
    expect(validatePlantForm({ ...base, species: { kind: 'free', text: '  ' } }, today)).toEqual({
      ok: true,
      value: base,
    });
    expect(
      validatePlantForm({ ...base, species: { kind: 'free', text: 'あ'.repeat(maxSpeciesFreeTextLength + 1) } }, today),
    ).toEqual({ ok: false, errors: { species: 'too_long' } });
  });

  it('購入日は今日まで（未来の日は future）', () => {
    expect(validatePlantForm({ ...base, acquiredOn: today }, today).ok).toBe(true);
    expect(validatePlantForm({ ...base, acquiredOn: '2026-11-21' }, today)).toEqual({
      ok: false,
      errors: { acquiredOn: 'future' },
    });
  });
});

describe('toPlantRequest', () => {
  it('種類がマスタなら speciesId、自由入力なら speciesFreeText、未設定なら speciesId を空文字で送る', () => {
    expect(toPlantRequest({ ...base, species: { kind: 'species', id: 'S1', name: 'ポトス' } })).toEqual({
      name: 'ポトス',
      speciesId: 'S1',
      locationId: '',
      acquiredOn: '',
    });
    expect(toPlantRequest({ ...base, species: { kind: 'free', text: '斑入りポトス' } })).toEqual({
      name: 'ポトス',
      speciesFreeText: '斑入りポトス',
      locationId: '',
      acquiredOn: '',
    });
    // 編集では、空文字で未設定に戻る（作成でも、空文字は未設定として扱われる）
    expect(toPlantRequest(base)).toEqual({ name: 'ポトス', speciesId: '', locationId: '', acquiredOn: '' });
  });
});

describe('plantToForm', () => {
  it('API の植物を入力欄の値にする', () => {
    expect(
      plantToForm({
        name: 'ポトス',
        speciesId: 'S1',
        speciesName: 'ポトス',
        locationId: 'L1',
        acquiredOn: '2026-04-01',
      }),
    ).toEqual({
      name: 'ポトス',
      species: { kind: 'species', id: 'S1', name: 'ポトス' },
      locationId: 'L1',
      acquiredOn: '2026-04-01',
    });
    expect(plantToForm({ name: 'a', speciesFreeText: '謎の多肉' })).toEqual({
      name: 'a',
      species: { kind: 'free', text: '謎の多肉' },
      locationId: '',
      acquiredOn: '',
    });
    expect(plantToForm({ name: 'a' }).species).toEqual({ kind: 'none' });
  });
});
