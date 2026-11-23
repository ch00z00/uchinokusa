import { summarizeBatch, toggleSelection } from '../batch';

const existing = {
  recordId: 'R0',
  actorDisplayName: 'たろう',
  performedOn: '2026-11-23',
  performedAt: '2026-11-23T01:00:00Z',
};

describe('summarizeBatch', () => {
  it('記録できた記録・すでに記録されていた件数・失敗したタスクに分ける', () => {
    const summary = summarizeBatch([
      { taskId: 'T1', status: 'completed', record: { id: 'R1' } },
      { taskId: 'T2', status: 'already_completed', existing },
      { taskId: 'T3', status: 'failed', error: { code: 'stale_cycle', message: 'x' } },
      { taskId: 'T4', status: 'completed', record: { id: 'R4' } },
    ] as Parameters<typeof summarizeBatch>[0]);
    expect(summary).toEqual({ recordIds: ['R1', 'R4'], alreadyCount: 1, failedTaskIds: ['T3'] });
  });

  it('知らない状態や、記録のない完了は失敗として扱う', () => {
    const summary = summarizeBatch([
      { taskId: 'T1', status: 'completed' },
      { taskId: 'T2', status: 'weird' },
    ] as unknown as Parameters<typeof summarizeBatch>[0]);
    expect(summary).toEqual({ recordIds: [], alreadyCount: 0, failedTaskIds: ['T1', 'T2'] });
  });
});

describe('toggleSelection', () => {
  it('選んでいなければ足し、選んでいれば外す（元の集合は変えない）', () => {
    const selected = new Set(['A']);
    expect([...toggleSelection(selected, 'B')]).toEqual(['A', 'B']);
    expect([...toggleSelection(selected, 'A')]).toEqual([]);
    expect([...selected]).toEqual(['A']);
  });
});
