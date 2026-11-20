import {
  emptyLocationForm,
  locationSummaryKeys,
  locationToForm,
  maxLocationNameLength,
  validateLocationForm,
  type LocationFormInput,
} from '../locationValidation';

const base: LocationFormInput = { ...emptyLocationForm };

describe('locationToForm', () => {
  it('未設定の項目は空文字にする', () => {
    expect(locationToForm({ name: '玄関', isIndoor: true })).toEqual({
      name: '玄関',
      isIndoor: true,
      windowDirection: '',
      windowDistance: '',
      shading: '',
    });
  });

  it('アプリが知らない値（あとから API に足された値）は未設定として扱う', () => {
    expect(
      locationToForm({
        name: 'ベランダ',
        isIndoor: false,
        windowDirection: 'up' as never,
        windowDistance: 'veryFar' as never,
        shading: 'blind' as never,
      }),
    ).toEqual({ name: 'ベランダ', isIndoor: false, windowDirection: '', windowDistance: '', shading: '' });
  });

  it('知っている値はそのまま', () => {
    expect(
      locationToForm({ name: '南窓', isIndoor: true, windowDirection: 's', windowDistance: 'near', shading: 'sheer' }),
    ).toEqual({ name: '南窓', isIndoor: true, windowDirection: 's', windowDistance: 'near', shading: 'sheer' });
  });
});

describe('validateLocationForm', () => {
  it('置き場所名の前後の空白（全角も）を除く', () => {
    expect(validateLocationForm({ ...base, name: '　リビングの南窓 ' })).toEqual({
      ok: true,
      value: { name: 'リビングの南窓', isIndoor: true, windowDirection: '', windowDistance: '', shading: '' },
    });
  });

  it('置き場所名が空なら required', () => {
    expect(validateLocationForm({ ...base, name: '  ' })).toEqual({ ok: false, errors: { name: 'required' } });
  });

  it('置き場所名は30文字まで（コードポイントで数える）', () => {
    expect(validateLocationForm({ ...base, name: '🌱'.repeat(maxLocationNameLength) }).ok).toBe(true);
    expect(validateLocationForm({ ...base, name: '🌱'.repeat(maxLocationNameLength + 1) })).toEqual({
      ok: false,
      errors: { name: 'too_long' },
    });
  });

  it('屋外では、窓からの距離と遮光を未設定に戻す（向きは残す）', () => {
    expect(
      validateLocationForm({
        name: 'ベランダ',
        isIndoor: false,
        windowDirection: 'se',
        windowDistance: 'near',
        shading: 'sheer',
      }),
    ).toEqual({
      ok: true,
      value: { name: 'ベランダ', isIndoor: false, windowDirection: 'se', windowDistance: '', shading: '' },
    });
  });
});

describe('locationSummaryKeys', () => {
  it('室内か屋外かと、設定済みの環境の項目を、この順に並べる', () => {
    expect(
      locationSummaryKeys({ isIndoor: true, windowDirection: 's', windowDistance: 'near', shading: 'sheer' }),
    ).toEqual([
      'location.indoor.true',
      'location.windowDirection.s',
      'location.windowDistance.near',
      'location.shading.sheer',
    ]);
  });

  it('未設定の項目と、知らない値は出さない', () => {
    expect(locationSummaryKeys({ isIndoor: false, windowDirection: 'up' as never })).toEqual(['location.indoor.false']);
  });

  it('屋外では、窓からの距離と遮光を出さない', () => {
    expect(
      locationSummaryKeys({ isIndoor: false, windowDirection: 'n', windowDistance: 'far', shading: 'none' }),
    ).toEqual(['location.indoor.false', 'location.windowDirection.n']);
  });
});
