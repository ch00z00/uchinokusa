import i18n from '@/i18n';

import { formatDay } from '@/lib/formatDate';

import {
  findInitialRepotting,
  findRepottingAvoid,
  repottingAvoidText,
  repottingReason,
  todaySuggestionText,
} from '../suggestionText';

const t = i18n.t.bind(i18n);

describe('repottingReason', () => {
  it('慣らし期間と、避けた時期を組み立てる', () => {
    expect(repottingReason({ afterAcclimation: true, acclimationDays: 14, avoided: ['midwinter'] }, t)).toBe(
      'お迎えから14日の慣らし期間のあと、真冬を避けた、最初の植え替えの適期です',
    );
  });

  it('避けた時期が2つなら「と」でつなぐ', () => {
    expect(repottingReason({ afterAcclimation: false, avoided: ['midsummer', 'midwinter'] }, t)).toBe(
      '真夏と真冬を避けた、最初の植え替えの適期です',
    );
  });

  it('種類ごとのルールを使ったときは、種類の名前を入れる', () => {
    expect(repottingReason({ afterAcclimation: false, avoided: [], species: 'ポトス' }, t)).toBe(
      'ポトスの植え替えに向いた時期のうち、最初の日です',
    );
  });

  it('知らない時期の値と、形の違う材料は無視する', () => {
    expect(repottingReason({ afterAcclimation: 'yes', avoided: ['spring', 3] }, t)).toBe('最初の植え替えの適期です');
  });
});

describe('findInitialRepotting', () => {
  const s = (extra: object) => ({
    id: 'S',
    providerId: 'repotting.v1',
    kind: 'do' as const,
    action: 'create_task' as const,
    messageKey: 'repotting.initial',
    messageParams: {},
    reasonKey: 'repotting.initial.reason',
    reasonParams: {},
    validFrom: '2026-11-20',
    status: 'pending' as const,
    ...extra,
  });

  it('その植物の、未回答の最初の植え替えの提案を返す', () => {
    const list = [s({ id: 'A', plantId: 'P2' }), s({ id: 'B', plantId: 'P1' })];
    expect(findInitialRepotting(list, 'P1')?.id).toBe('B');
  });

  it('ほかの提案や、回答済みのものは返さない', () => {
    const list = [
      s({ id: 'A', plantId: 'P1', messageKey: 'repotting.avoid' }),
      s({ id: 'B', plantId: 'P1', status: 'dismissed' }),
    ];
    expect(findInitialRepotting(list, 'P1')).toBeUndefined();
  });
});

describe('findRepottingAvoid', () => {
  const avoid = (extra: object) => ({
    id: 'S',
    providerId: 'repotting.v1',
    kind: 'avoid' as const,
    action: 'info' as const,
    taskId: 'T1',
    messageKey: 'repotting.avoid',
    messageParams: { plantName: 'ポトス', date: '2027-01-10' },
    reasonKey: 'repotting.avoid.midwinter.reason',
    reasonParams: {},
    validFrom: '2026-11-22',
    status: 'pending' as const,
    ...extra,
  });

  it('そのタスクへの、未回答の植え替えの時期の注意を返す', () => {
    const list = [avoid({ id: 'A', taskId: 'T2' }), avoid({ id: 'B' })];
    expect(findRepottingAvoid(list, 'T1')?.id).toBe('B');
  });

  it('ほかの提案や、回答済みのものは返さない', () => {
    const list = [avoid({ id: 'A', messageKey: 'repotting.initial' }), avoid({ id: 'B', status: 'dismissed' })];
    expect(findRepottingAvoid(list, 'T1')).toBeUndefined();
  });

  it('本文に日付、理由に時期ごとの文を入れる', () => {
    const text = repottingAvoidText(avoid({}), t);
    expect(text.message).toBe(`${formatDay('2027-01-10')}は植え替えに向かない時期です`);
    expect(text.reason).toBe('寒さで根が傷みやすくなります');
    expect(repottingAvoidText(avoid({ reasonKey: 'repotting.avoid.midsummer.reason' }), t).reason).toBe(
      '暑さで生長が鈍り、植え替えのあと回復しにくくなります',
    );
    expect(repottingAvoidText(avoid({ reasonKey: 'repotting.avoid.avoid.reason' }), t).reason).toBe(
      'この植物の種類の植え替えには向きません',
    );
  });

  it('知らない理由のキーや、日付のない材料でも文を作る', () => {
    const text = repottingAvoidText(avoid({ reasonKey: 'repotting.avoid.rainy.reason', messageParams: {} }), t);
    expect(text.message).toBe('予定日は植え替えに向かない時期です');
    expect(text.reason).toBe('株を傷めやすくなります');
  });
});

describe('todaySuggestionText', () => {
  const base = {
    id: 'S',
    providerId: 'x',
    action: 'info' as const,
    messageParams: {},
    reasonParams: {},
    validFrom: '2026-11-23',
    status: 'pending' as const,
  };

  it('植え替えの時期の注意は、本文と理由をつなぐ', () => {
    const text = todaySuggestionText(
      {
        ...base,
        kind: 'avoid',
        messageKey: 'repotting.avoid',
        messageParams: { date: '2027-01-10' },
        reasonKey: 'repotting.avoid.midwinter.reason',
      },
      t,
    );
    expect(text).toBe(`${formatDay('2027-01-10')}は植え替えに向かない時期です。寒さで根が傷みやすくなります`);
  });

  it('知らない提案は、種類ごとの一般的な文にする。条件つきは条件を入れる', () => {
    expect(todaySuggestionText({ ...base, kind: 'avoid', messageKey: 'weather.rain', reasonKey: 'x' }, t)).toBe(
      '今日はやらない方がよさそうです',
    );
    expect(
      todaySuggestionText(
        { ...base, kind: 'conditional', messageKey: 'weather.x', reasonKey: 'x', conditionText: '土が湿っていたら' },
        t,
      ),
    ).toBe('もし土が湿っていたら、今日は見送ってください');
  });
});
