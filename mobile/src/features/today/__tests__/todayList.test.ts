import { groupTodayItems, parseGrouping, todaySuggestion, type TodayItem } from '../todayList';

const item = (id: string, extra: { kind?: string; location?: string; suggestions?: TodayItem['suggestions'] } = {}) =>
  ({
    task: {
      id,
      plantId: `P${id}`,
      kind: extra.kind ?? 'watering',
      interval: { value: 7, unit: 'day' },
      seasonalIntervals: {},
      cycleSeq: 1,
      status: 'active',
      createdAt: '2026-11-01T00:00:00Z',
      nextDueOn: '2026-11-23',
    },
    plant: {
      id: `P${id}`,
      name: `植物${id}`,
      ...(extra.location ? { locationId: `L-${extra.location}`, locationName: extra.location } : {}),
    },
    overdueDays: 0,
    suggestions: extra.suggestions ?? [],
  }) as TodayItem;

const suggestion = (kind: 'avoid' | 'conditional' | 'do', id = kind) => ({
  id,
  providerId: 'x',
  kind,
  action: 'info' as const,
  messageKey: 'x',
  messageParams: {},
  reasonKey: 'x',
  reasonParams: {},
  validFrom: '2026-11-23',
  status: 'pending' as const,
});

describe('groupTodayItems', () => {
  const items = [
    item('1', { kind: 'fertilizing', location: '寝室' }),
    item('2', { kind: 'watering' }),
    item('3', { kind: 'watering', location: 'リビング' }),
    item('4', { kind: 'other', location: '寝室' }),
  ];

  it('予定日順は、API の順のまま見出しなしの1つにする', () => {
    expect(groupTodayItems(items, 'date', '置き場所未設定')).toEqual([{ key: 'all', items }]);
  });

  it('置き場所ごとは、置き場所名の順で、未設定を最後にする。中の順は API の順', () => {
    const sections = groupTodayItems(items, 'location', '置き場所未設定');
    expect(sections.map((s) => [s.title, s.items.map((i) => i.task.id)])).toEqual([
      ['リビング', ['3']],
      ['寝室', ['1', '4']],
      ['置き場所未設定', ['2']],
    ]);
  });

  it('作業の種類ごとは、水やり・肥料・植え替え・剪定・その他の順。知らない種類は「その他」', () => {
    const sections = groupTodayItems([...items, item('5', { kind: 'misting' })], 'kind', '置き場所未設定');
    expect(sections.map((s) => [s.key, s.items.map((i) => i.task.id)])).toEqual([
      ['watering', ['2', '3']],
      ['fertilizing', ['1']],
      ['other', ['4', '5']],
    ]);
  });
});

describe('todaySuggestion', () => {
  it('「やらない方がよい」を「条件つき」より先に選ぶ。「おすすめ」は出さない', () => {
    expect(todaySuggestion(item('1', { suggestions: [suggestion('conditional'), suggestion('avoid')] }))?.kind).toBe(
      'avoid',
    );
    expect(todaySuggestion(item('1', { suggestions: [suggestion('conditional')] }))?.kind).toBe('conditional');
    expect(todaySuggestion(item('1', { suggestions: [suggestion('do')] }))).toBeUndefined();
    expect(todaySuggestion(item('1'))).toBeUndefined();
  });
});

describe('parseGrouping', () => {
  it('保存した値を読む。知らない値や空は予定日順', () => {
    expect(parseGrouping('location')).toBe('location');
    expect(parseGrouping('kind')).toBe('kind');
    expect(parseGrouping('weird')).toBe('date');
    expect(parseGrouping(null)).toBe('date');
  });
});
