import { currentInterval, normalizeCareKind, overdueDays, seasonOf } from '../careTask';

describe('normalizeCareKind', () => {
  it('知っている種類はそのまま、知らない値は「その他」にする（API の決まり）', () => {
    expect(normalizeCareKind('watering')).toBe('watering');
    expect(normalizeCareKind('misting')).toBe('other');
  });
});

describe('seasonOf', () => {
  it('北半球の季節：春 3〜5月、夏 6〜8月、秋 9〜11月、冬 12〜2月', () => {
    expect(seasonOf(new Date(2026, 2, 1))).toBe('spring');
    expect(seasonOf(new Date(2026, 4, 31))).toBe('spring');
    expect(seasonOf(new Date(2026, 5, 1))).toBe('summer');
    expect(seasonOf(new Date(2026, 8, 1))).toBe('autumn');
    expect(seasonOf(new Date(2026, 10, 30))).toBe('autumn');
    expect(seasonOf(new Date(2026, 11, 1))).toBe('winter');
    expect(seasonOf(new Date(2027, 1, 28))).toBe('winter');
  });
});

describe('currentInterval', () => {
  const base = { value: 7, unit: 'day' as const };
  it('今の季節の周期があればそれを、なければ基本の周期を使う', () => {
    const task = { interval: base, seasonalIntervals: { summer: { value: 4, unit: 'day' as const } } };
    expect(currentInterval(task, new Date(2026, 6, 1))).toEqual({
      interval: { value: 4, unit: 'day' },
      season: 'summer',
    });
    expect(currentInterval(task, new Date(2026, 0, 1))).toEqual({ interval: base, season: undefined });
  });
});

describe('overdueDays', () => {
  const task = { status: 'active' as const, nextDueOn: '2026-11-21' };
  it('次の予定日を過ぎた日数。今日が予定日なら 0（今日やることと同じく、過ぎてはいない）', () => {
    expect(overdueDays(task, '2026-11-24')).toBe(3);
    expect(overdueDays(task, '2026-11-21')).toBe(0);
    expect(overdueDays(task, '2026-11-20')).toBe(0);
  });

  it('月をまたいでも数える', () => {
    expect(overdueDays({ ...task, nextDueOn: '2026-10-30' }, '2026-11-02')).toBe(3);
  });

  it('一時停止中・予定日がないタスクは、期限切れにしない（今日やることに出ないため）', () => {
    expect(overdueDays({ ...task, status: 'paused' }, '2026-11-24')).toBe(0);
    expect(overdueDays({ status: 'active', nextDueOn: undefined }, '2026-11-24')).toBe(0);
  });
});
