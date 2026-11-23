import {
  actorOf,
  findRecord,
  flattenRecords,
  historyFilters,
  kindParam,
  validateRecordDate,
  type CareRecord,
} from '../careRecord';

const record = (id: string, over: Partial<CareRecord> = {}): CareRecord => ({
  id,
  plantId: 'p1',
  taskId: 't1',
  kind: 'watering',
  outcome: 'completed',
  performedOn: '2026-11-23',
  performedAt: '2026-11-23T01:00:00Z',
  actorUserId: 'u1',
  actorDisplayName: 'ゆうすけ',
  cycleSeq: 1,
  isExtra: false,
  createdAt: '2026-11-23T01:00:00Z',
  ...over,
});

describe('flattenRecords', () => {
  it('ページをつないで1つの並びにする', () => {
    const pages = [
      { records: [record('a'), record('b')], lastPerformed: [] },
      { records: [record('c')], lastPerformed: [] },
    ];
    expect(flattenRecords(pages).map((r) => r.id)).toEqual(['a', 'b', 'c']);
  });

  it('読み込みのあいだに新しい記録が入ってページがずれても、同じ記録を2回出さない', () => {
    const pages = [
      { records: [record('a'), record('b')], lastPerformed: [] },
      { records: [record('b'), record('c')], lastPerformed: [] },
    ];
    expect(flattenRecords(pages).map((r) => r.id)).toEqual(['a', 'b', 'c']);
  });

  it('まだ読み込んでいなければ空', () => {
    expect(flattenRecords(undefined)).toEqual([]);
  });
});

describe('findRecord', () => {
  it('取得した結果のどれかのページから探す', () => {
    const pages = [[{ records: [record('a')], lastPerformed: [] }], [{ records: [record('b')], lastPerformed: [] }]];
    expect(findRecord(pages, 'b')?.id).toBe('b');
    expect(findRecord(pages, 'z')).toBeUndefined();
  });
});

describe('actorOf', () => {
  it('アカウントがあれば記録の時点の名前、削除していれば退会したユーザー', () => {
    expect(actorOf(record('a'))).toEqual({ status: 'member', name: 'ゆうすけ' });
    expect(actorOf(record('a', { actorUserId: undefined }))).toEqual({ status: 'deleted' });
  });
});

describe('validateRecordDate', () => {
  it('未来の日は選べない。今日までならよい', () => {
    expect(validateRecordDate('2026-11-25', '2026-11-24')).toBe('future');
    expect(validateRecordDate('2026-11-24', '2026-11-24')).toBeUndefined();
    expect(validateRecordDate('2026-10-01', '2026-11-24')).toBeUndefined();
  });
});

describe('kindParam', () => {
  it('「すべて」なら絞り込まない', () => {
    expect(historyFilters[0]).toBe('all');
    expect(kindParam('all')).toBeUndefined();
    expect(kindParam('fertilizing')).toBe('fertilizing');
  });
});
