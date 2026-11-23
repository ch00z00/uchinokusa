import { ApiError } from '@/api/errors';

import { classifyCompletionError, withoutTask } from '../completion';
import type { TodayList } from '../todayList';

const list = {
  date: '2026-11-23',
  items: [{ task: { id: 'T1' } }, { task: { id: 'T2' } }],
} as unknown as TodayList;

describe('withoutTask', () => {
  it('完了したタスクを一覧から外す（元の一覧は変えない）', () => {
    expect(withoutTask(list, 'T1').items.map((i) => i.task.id)).toEqual(['T2']);
    expect(list.items).toHaveLength(2);
  });
});

describe('classifyCompletionError', () => {
  const existing = {
    recordId: 'R',
    actorDisplayName: 'たろう',
    performedOn: '2026-11-23',
    performedAt: '2026-11-23T01:15:00Z',
  };

  it('すでに完了していれば、誰がいつ完了したかを返す', () => {
    const error = new ApiError({
      status: 409,
      code: 'already_completed',
      message: 'x',
      body: { code: 'already_completed', message: 'x', existing },
    });
    expect(classifyCompletionError(error)).toEqual({ type: 'already_completed', existing });
  });

  it('先に完了した記録が本文になければ、ほかのエラーとして扱う', () => {
    const error = new ApiError({ status: 409, code: 'already_completed', message: 'x', body: {} });
    expect(classifyCompletionError(error).type).toBe('other');
  });

  it('予定の回が合わない・通信できない・そのほかを見分ける', () => {
    expect(classifyCompletionError(new ApiError({ status: 409, code: 'stale_cycle', message: 'x' })).type).toBe(
      'stale',
    );
    expect(classifyCompletionError(new ApiError({ status: null, code: 'network', message: 'x' })).type).toBe('network');
    expect(classifyCompletionError(new Error('x')).type).toBe('other');
  });
});
