import { useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import type { components } from '@/api/schema';
import { plantKeys } from '@/features/plant/usePlants';

import { withoutTask } from './completion';
import type { TodayList } from './todayList';

const todayPrefix = (householdId: string) => ['households', householdId, 'today'] as const;

/** 世帯の取得をまとめて取り直す（今日やること・植物の一覧の次の予定日・植物の詳細のタスク） */
const refreshHousehold = (queryClient: QueryClient, householdId: string) =>
  Promise.all([
    queryClient.invalidateQueries({ queryKey: ['households', householdId] }),
    queryClient.invalidateQueries({ queryKey: plantKeys.details }),
  ]);

type CompleteVariables = { taskId: string; cycleSeq: number; allowExtra?: boolean; performedOn?: string };

/**
 * 今日やることの一覧から、そのタスクを楽観的に外す（完了・スキップの共通）。失敗したら元に戻せるように、外す前の一覧を返す
 */
async function removeFromToday(queryClient: QueryClient, householdId: string, taskIds: readonly string[]) {
  // 取得の途中の結果で、外した項目が戻らないように止める
  await queryClient.cancelQueries({ queryKey: todayPrefix(householdId) });
  const snapshot = queryClient.getQueriesData<TodayList>({ queryKey: todayPrefix(householdId) });
  queryClient.setQueriesData<TodayList>({ queryKey: todayPrefix(householdId) }, (list) =>
    list ? taskIds.reduce(withoutTask, list) : list,
  );
  return { snapshot };
}

function restoreToday(queryClient: QueryClient, context: Awaited<ReturnType<typeof removeFromToday>> | undefined) {
  for (const [key, data] of context?.snapshot ?? []) queryClient.setQueryData(key, data);
}

/**
 * タスクの完了（POST /tasks/{id}/completions。編集者以上。要件10-1・10-3・10-4・11-2）。
 * 押した直後に、今日やることの一覧から外す（楽観的更新）。失敗したら一覧を元に戻す（理由の知らせは画面で行う）
 */
export function useCompleteTask(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ taskId, cycleSeq, allowExtra, performedOn }: CompleteVariables) =>
      unwrap(
        api.POST('/tasks/{taskId}/completions', {
          params: { path: { taskId } },
          body: { cycleSeq, ...(allowExtra ? { allowExtra } : {}), ...(performedOn ? { performedOn } : {}) },
        }),
      ),
    onMutate: ({ taskId }) => removeFromToday(queryClient, householdId, [taskId]),
    onError: (_error, _variables, context) => restoreToday(queryClient, context),
    onSettled: () => refreshHousehold(queryClient, householdId),
  });
}

/** タスクのスキップ（POST /tasks/{id}/skips。編集者以上。要件10-6〜10-8）。完了と同じく、一覧から楽観的に外す */
export function useSkipTask(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ taskId, body }: { taskId: string; body: components['schemas']['SkipCareTaskRequest'] }) =>
      unwrap(api.POST('/tasks/{taskId}/skips', { params: { path: { taskId } }, body })),
    onMutate: ({ taskId }) => removeFromToday(queryClient, householdId, [taskId]),
    onError: (_error, _variables, context) => restoreToday(queryClient, context),
    onSettled: () => refreshHousehold(queryClient, householdId),
  });
}

/**
 * まとめて完了（POST /households/{id}/completions:batch。編集者以上。要件10-9）。
 * 選んだタスクを一覧から楽観的に外す。項目ごとの失敗は結果で返り、取り直しで一覧に戻る
 */
export function useCompleteTasks(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (items: components['schemas']['CompleteCareTasksItem'][]) =>
      unwrap(
        api.POST('/households/{householdId}/completions:batch', {
          params: { path: { householdId } },
          body: { items },
        }),
      ),
    onMutate: (items) =>
      removeFromToday(
        queryClient,
        householdId,
        items.map((i) => i.taskId),
      ),
    onError: (_error, _variables, context) => restoreToday(queryClient, context),
    onSettled: () => refreshHousehold(queryClient, householdId),
  });
}

/** 記録の取り消し（POST /records/{id}/undo。記録した本人が10分以内。要件10-4） */
export function useUndoRecord(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (recordId: string) => unwrap(api.POST('/records/{recordId}/undo', { params: { path: { recordId } } })),
    onSettled: () => refreshHousehold(queryClient, householdId),
  });
}
