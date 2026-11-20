import i18n from '@/i18n';

import { findInitialRepotting, repottingReason } from '../suggestionText';

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
