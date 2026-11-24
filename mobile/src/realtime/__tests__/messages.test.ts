import { checkVersion, invalidationTargets, parseMessage, reconnectDelay, type ChangedMessage } from '../messages';

describe('parseMessage', () => {
  it('知っている種類を読み取る', () => {
    expect(parseMessage('{"type":"subscribed","householdIds":["h1"]}')).toEqual({
      type: 'subscribed',
      householdIds: ['h1'],
    });
    expect(parseMessage('{"type":"role_changed","householdId":"h1","role":"viewer"}')).toEqual({
      type: 'role_changed',
      householdId: 'h1',
      role: 'viewer',
    });
    expect(
      parseMessage(
        '{"type":"changed","householdId":"h1","version":3,"entity":"task","entityId":"t1","op":"updated","actorUserId":"u1","plantId":"p1"}',
      ),
    ).toMatchObject({ type: 'changed', version: 3, entity: 'task', plantId: 'p1' });
  });

  it('読めない・知らない種類・項目の欠けたメッセージは無視する（新しいサーバーとの組み合わせのため）', () => {
    expect(parseMessage('not json')).toBeNull();
    expect(parseMessage('{"type":"new_kind"}')).toBeNull();
    expect(parseMessage('{"type":"changed","householdId":"h1"}')).toBeNull();
  });
});

describe('checkVersion', () => {
  it('はじめての番号は、そのまま受け取る（購読の直後は手元の番号がない）', () => {
    expect(checkVersion(undefined, 5)).toBe('apply');
  });

  it('次の番号なら受け取り、古い番号は無視し、2つ以上飛んだら取りこぼしとする', () => {
    expect(checkVersion(5, 6)).toBe('apply');
    expect(checkVersion(5, 5)).toBe('stale');
    expect(checkVersion(5, 4)).toBe('stale');
    expect(checkVersion(5, 7)).toBe('gap');
  });
});

const changed = (over: Partial<ChangedMessage>): ChangedMessage => ({
  type: 'changed',
  householdId: 'h1',
  version: 1,
  entity: 'task',
  entityId: 'e1',
  op: 'updated',
  ...over,
});

describe('invalidationTargets', () => {
  it('世帯のデータ（今日やること・植物の一覧・提案など）と、植物に属するデータなら植物の詳細を取り直す', () => {
    expect(invalidationTargets(changed({ entity: 'record', plantId: 'p1' }))).toEqual([
      { queryKey: ['households', 'h1'] },
      { queryKey: ['plants', 'p1'] },
    ]);
  });

  it('植物そのものの変更は、その植物の詳細を取り直す', () => {
    expect(invalidationTargets(changed({ entity: 'plant', entityId: 'p2' }))).toEqual([
      { queryKey: ['households', 'h1'] },
      { queryKey: ['plants', 'p2'] },
    ]);
  });

  it('置き場所の変更は、植物の詳細すべて（置き場所の名前を出す）を取り直す', () => {
    expect(invalidationTargets(changed({ entity: 'location' }))).toEqual([
      { queryKey: ['households', 'h1'] },
      { queryKey: ['plants'] },
    ]);
  });

  it('メンバーの削除は、記録・メモの作業者名が変わるので、植物の詳細すべても取り直す（0035）', () => {
    expect(invalidationTargets(changed({ entity: 'member', op: 'deleted' }))).toEqual([
      { queryKey: ['households', 'h1'] },
      { queryKey: ['plants'] },
      { queryKey: ['households'], exact: true },
    ]);
  });

  it('世帯とメンバーの変更は、世帯の一覧（名前・ロール）も取り直す', () => {
    expect(invalidationTargets(changed({ entity: 'member' }))).toEqual([
      { queryKey: ['households', 'h1'] },
      { queryKey: ['households'], exact: true },
    ]);
    expect(invalidationTargets(changed({ entity: 'household' }))).toEqual([
      { queryKey: ['households', 'h1'] },
      { queryKey: ['households'], exact: true },
    ]);
  });
});

describe('reconnectDelay', () => {
  it('1秒から倍にしながら、30秒まで広げる', () => {
    expect([0, 1, 2, 3, 4, 5, 6].map(reconnectDelay)).toEqual([1000, 2000, 4000, 8000, 16000, 30000, 30000]);
  });
});
