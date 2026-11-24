import type { CareRecord } from '@/features/record/careRecord';

import {
  calendarDays,
  dayDetail,
  initialSelectedDate,
  markerLimit,
  shiftMonth,
  visibleMarkers,
  type PlantCalendar,
} from '../calendarDays';

const record = (id: string, over: Partial<CareRecord> = {}): CareRecord => ({
  id,
  plantId: 'p1',
  taskId: 't1',
  kind: 'watering',
  outcome: 'completed',
  performedOn: '2026-11-03',
  performedAt: '2026-11-03T01:00:00Z',
  actorUserId: 'u1',
  actorDisplayName: 'ゆうすけ',
  cycleSeq: 1,
  isExtra: false,
  createdAt: '2026-11-03T01:00:00Z',
  ...over,
});

const note = (id: string, date: string) => ({
  date,
  note: { id, plantId: 'p1', body: 'メモ', authorDisplayName: 'ゆうすけ', createdAt: `${date}T01:00:00Z` },
});

const calendar: PlantCalendar = {
  month: '2026-11',
  records: [
    record('r1'),
    record('r2', { kind: 'fertilizing' }),
    record('r3', { outcome: 'skipped', performedOn: '2026-11-10' }),
    // 同じ日に同じ種類を2回（追加の記録）。印は1つにまとめる
    record('r4', { isExtra: true }),
  ],
  plans: [
    { date: '2026-11-10', projected: false, taskId: 't1', kind: 'watering' },
    { date: '2026-11-17', projected: true, taskId: 't1', kind: 'watering' },
  ],
  notes: [note('n1', '2026-11-03'), note('n2', '2026-11-20')],
};

describe('calendarDays', () => {
  it('日ごとに、完了・スキップ・予定の順で印を並べ、同じ種類と状態は1つにまとめる', () => {
    const days = calendarDays(calendar);
    expect(days.get('2026-11-03')?.markers).toEqual([
      { kind: 'watering', status: 'completed' },
      { kind: 'fertilizing', status: 'completed' },
    ]);
    expect(days.get('2026-11-10')?.markers).toEqual([
      { kind: 'watering', status: 'skipped' },
      { kind: 'watering', status: 'planned' },
    ]);
    expect(days.get('2026-11-17')?.markers).toEqual([{ kind: 'watering', status: 'planned' }]);
  });

  it('メモを書いた日に印を付ける', () => {
    const days = calendarDays(calendar);
    expect(days.get('2026-11-03')?.hasNote).toBe(true);
    expect(days.get('2026-11-20')).toEqual({ markers: [], hasNote: true });
    expect(days.get('2026-11-17')?.hasNote).toBe(false);
  });
});

describe('visibleMarkers', () => {
  it('入りきらなければ、最後の枠を「+n」にする', () => {
    const markers = Array.from({ length: markerLimit + 2 }, () => ({
      kind: 'watering' as const,
      status: 'completed' as const,
    }));
    const { shown, more } = visibleMarkers(markers);
    expect(shown).toHaveLength(markerLimit - 1);
    expect(more).toBe(3);
    expect(visibleMarkers(markers.slice(0, markerLimit))).toEqual({ shown: markers.slice(0, markerLimit), more: 0 });
  });
});

describe('dayDetail', () => {
  it('選んだ日の記録・予定・メモを返す', () => {
    const detail = dayDetail(calendar, '2026-11-03');
    expect(detail.records.map((r) => r.id)).toEqual(['r1', 'r2', 'r4']);
    expect(detail.plans).toEqual([]);
    expect(detail.notes.map((n) => n.id)).toEqual(['n1']);
  });
});

describe('shiftMonth・initialSelectedDate', () => {
  it('前月・翌月へ年をまたいで動く', () => {
    expect(shiftMonth('2026-12', 1)).toBe('2027-01');
    expect(shiftMonth('2026-01', -1)).toBe('2025-12');
  });

  it('今日がその月なら今日、ほかの月なら1日を選ぶ', () => {
    expect(initialSelectedDate('2026-11', '2026-11-24')).toBe('2026-11-24');
    expect(initialSelectedDate('2026-12', '2026-11-24')).toBe('2026-12-01');
  });
});
