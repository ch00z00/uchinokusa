import { ApiError } from '@/api/errors';
import type { components } from '@/api/schema';

import type { TodayList } from './todayList';

export type CompletedBy = components['schemas']['CompletedBy'];

/** 完了したタスクを、今日やることの一覧から外す（楽観的更新。元の一覧は変えない） */
export const withoutTask = (list: TodayList, taskId: string): TodayList => ({
  ...list,
  items: list.items.filter((item) => item.task.id !== taskId),
});

export type CompletionFailure =
  /** 同じ回を、ほかのメンバーが先に完了していた（TODAY-05。要件11-2） */
  | { type: 'already_completed'; existing: CompletedBy }
  /** 予定の回が合わない（タスクが変わった）。一覧を取り直す */
  | { type: 'stale' }
  /** 通信できなかった（要件20-5） */
  | { type: 'network' }
  | { type: 'other'; error: unknown };

const isCompletedBy = (value: unknown): value is CompletedBy =>
  typeof value === 'object' &&
  value !== null &&
  typeof (value as CompletedBy).actorDisplayName === 'string' &&
  typeof (value as CompletedBy).performedAt === 'string';

/** 完了の失敗を、画面での扱いごとに見分ける */
export function classifyCompletionError(error: unknown): CompletionFailure {
  if (!(error instanceof ApiError)) return { type: 'other', error };
  if (error.code === 'already_completed') {
    const existing = (error.body as { existing?: unknown } | undefined)?.existing;
    if (isCompletedBy(existing)) return { type: 'already_completed', existing };
  }
  if (error.code === 'stale_cycle') return { type: 'stale' };
  if (error.code === 'network') return { type: 'network' };
  return { type: 'other', error };
}
