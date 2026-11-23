import type { components } from '@/api/schema';

type ItemResult = components['schemas']['CompleteCareTasksItemResult'];

export type BatchSummary = {
  /** 記録できた記録の ID（取り消しに使う） */
  recordIds: string[];
  /** ほかのメンバーが先に記録していた件数 */
  alreadyCount: number;
  /** 記録できなかったタスク（一覧に戻し、選んだままにする） */
  failedTaskIds: string[];
};

/** まとめて完了の結果（TODAY-06、要件10-9）を、画面での扱いごとに分ける。知らない状態は失敗として扱う */
export function summarizeBatch(results: readonly ItemResult[]): BatchSummary {
  const summary: BatchSummary = { recordIds: [], alreadyCount: 0, failedTaskIds: [] };
  for (const result of results) {
    if (result.status === 'completed' && result.record) summary.recordIds.push(result.record.id);
    else if (result.status === 'already_completed') summary.alreadyCount += 1;
    else summary.failedTaskIds.push(result.taskId);
  }
  return summary;
}

/** 選択モードで、タスクを選ぶ・外す（元の集合は変えない） */
export function toggleSelection(selected: ReadonlySet<string>, taskId: string): Set<string> {
  const next = new Set(selected);
  if (next.has(taskId)) next.delete(taskId);
  else next.add(taskId);
  return next;
}
