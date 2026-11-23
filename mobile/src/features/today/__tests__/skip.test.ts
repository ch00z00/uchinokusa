import { emptySkipForm, toSkipRequest, validateSkip } from '../skip';

const tomorrow = '2026-11-24';

describe('validateSkip', () => {
  it('初期値は理由なし・確認日は明日で、そのまま送れる', () => {
    expect(validateSkip(emptySkipForm(tomorrow), tomorrow)).toEqual({});
  });

  it('「その他」の自由入力は200文字まで', () => {
    const form = { ...emptySkipForm(tomorrow), reason: 'other' as const, reasonText: 'あ'.repeat(201) };
    expect(validateSkip(form, tomorrow)).toEqual({ reasonText: 'too_long' });
    expect(validateSkip({ ...form, reasonText: 'あ'.repeat(200) }, tomorrow)).toEqual({});
  });

  it('確認日は明日以降', () => {
    expect(validateSkip({ ...emptySkipForm(tomorrow), recheckOn: '2026-11-23' }, tomorrow)).toEqual({
      recheckOn: 'past',
    });
  });
});

describe('toSkipRequest', () => {
  it('理由がなければ送らない。自由入力は「その他」のときだけ、前後の空白を除いて送る', () => {
    expect(toSkipRequest(emptySkipForm(tomorrow), 3)).toEqual({ cycleSeq: 3, recheckOn: tomorrow });
    expect(toSkipRequest({ reason: 'soil_wet', reasonText: '残り', recheckOn: tomorrow }, 3)).toEqual({
      cycleSeq: 3,
      reason: 'soil_wet',
      recheckOn: tomorrow,
    });
    expect(toSkipRequest({ reason: 'other', reasonText: ' 旅行中 ', recheckOn: '2026-11-30' }, 3)).toEqual({
      cycleSeq: 3,
      reason: 'other',
      reasonText: '旅行中',
      recheckOn: '2026-11-30',
    });
    expect(toSkipRequest({ reason: 'other', reasonText: '  ', recheckOn: tomorrow }, 3)).toEqual({
      cycleSeq: 3,
      reason: 'other',
      recheckOn: tomorrow,
    });
  });
});
