import { maxHouseholdNameLength, maxRegionLength, validateHouseholdForm } from '../householdValidation';

describe('validateHouseholdForm', () => {
  it('前後の空白を除いた値を返す。地域の空は未設定（空文字）', () => {
    expect(validateHouseholdForm({ name: '　自宅 ', regionPrefecture: '', regionCity: '  ' })).toEqual({
      ok: true,
      value: { name: '自宅', regionPrefecture: '', regionCity: '' },
    });
  });

  it('世帯名が空なら required', () => {
    expect(validateHouseholdForm({ name: '  ', regionPrefecture: '', regionCity: '' })).toEqual({
      ok: false,
      errors: { name: 'required' },
    });
  });

  it('世帯名は30文字まで（コードポイントで数える）', () => {
    const ok = validateHouseholdForm({
      name: '🌱'.repeat(maxHouseholdNameLength),
      regionPrefecture: '',
      regionCity: '',
    });
    expect(ok.ok).toBe(true);
    expect(
      validateHouseholdForm({ name: 'あ'.repeat(maxHouseholdNameLength + 1), regionPrefecture: '', regionCity: '' }),
    ).toEqual({ ok: false, errors: { name: 'too_long' } });
  });

  it('市区町村は50文字まで', () => {
    expect(
      validateHouseholdForm({ name: '自宅', regionPrefecture: '東京都', regionCity: 'あ'.repeat(maxRegionLength + 1) }),
    ).toEqual({ ok: false, errors: { regionCity: 'too_long' } });
  });

  it('複数の誤りは、まとめて返す', () => {
    expect(
      validateHouseholdForm({ name: '', regionPrefecture: '', regionCity: 'あ'.repeat(maxRegionLength + 1) }),
    ).toEqual({ ok: false, errors: { name: 'required', regionCity: 'too_long' } });
  });
});
