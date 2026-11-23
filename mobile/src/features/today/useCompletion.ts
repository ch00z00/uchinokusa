import { useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
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

type CompleteVariables = { taskId: string; cycleSeq: number; allowExtra?: boolean };

/**
 * タスクの完了（POST /tasks/{id}/completions。編集者以上。要件10-1・10-4・11-2）。
 * 押した直後に、今日やることの一覧から外す（楽観的更新）。失敗したら一覧を元に戻す（理由の知らせは画面で行う）
 */
export function useCompleteTask(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ taskId, cycleSeq, allowExtra }: CompleteVariables) =>
      unwrap(
        api.POST('/tasks/{taskId}/completions', {
          params: { path: { taskId } },
          body: { cycleSeq, ...(allowExtra ? { allowExtra } : {}) },
        }),
      ),
    onMutate: async ({ taskId }) => {
      // 取得の途中の結果で、外した項目が戻らないように止める
      await queryClient.cancelQueries({ queryKey: todayPrefix(householdId) });
      const snapshot = queryClient.getQueriesData<TodayList>({ queryKey: todayPrefix(householdId) });
      queryClient.setQueriesData<TodayList>({ queryKey: todayPrefix(householdId) }, (list) =>
        list ? withoutTask(list, taskId) : list,
      );
      return { snapshot };
    },
    onError: (_error, _variables, context) => {
      for (const [key, data] of context?.snapshot ?? []) queryClient.setQueryData(key, data);
    },
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
