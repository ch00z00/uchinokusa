import { useInfiniteQuery, useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import { plantKeys } from '@/features/plant/usePlants';

import { kindParam, type CareRecordPage, type HistoryFilter } from './careRecord';

export const recordKeys = {
  /**
   * 植物の履歴すべて（絞り込みごと）。植物の詳細のキーの下に置き、完了・スキップ（今日やること）や
   * タスクの変更で植物を取り直すときに、いっしょに取り直す
   */
  all: (plantId: string) => [...plantKeys.detail(plantId), 'records'] as const,
  list: (plantId: string, filter: HistoryFilter) => [...recordKeys.all(plantId), { filter }] as const,
};

const pageSize = 30;

/** 植物の履歴（GET /plants/{id}/records。新しい順、30件ずつ。閲覧者も見られる。要件12-1〜12-3） */
export function useRecords(plantId: string, filter: HistoryFilter) {
  return useInfiniteQuery({
    queryKey: recordKeys.list(plantId, filter),
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }): Promise<CareRecordPage> =>
      unwrap(
        api.GET('/plants/{plantId}/records', {
          params: { path: { plantId }, query: { kind: kindParam(filter), cursor: pageParam, limit: pageSize } },
        }),
      ),
    getNextPageParam: (last) => last.nextCursor,
  });
}

/**
 * 記録が変わったら、植物（タスクの次の予定日・履歴）と世帯（今日やること・植物の一覧の次の予定日）を取り直す。
 * いちばん新しい完了を変える・消すと、サーバーが次の予定日を計算し直すため（要件12-4）
 */
const invalidateRecordChange = (queryClient: QueryClient, householdId: string, plantId: string) =>
  Promise.all([
    queryClient.invalidateQueries({ queryKey: plantKeys.detail(plantId) }),
    queryClient.invalidateQueries({ queryKey: ['households', householdId] }),
  ]);

/** 記録の日付の変更（PATCH /records/{id}。編集者以上。要件12-4） */
export function useUpdateRecord(householdId: string, plantId: string, recordId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (performedOn: string) =>
      unwrap(api.PATCH('/records/{recordId}', { params: { path: { recordId } }, body: { performedOn } })),
    onSuccess: () => invalidateRecordChange(queryClient, householdId, plantId),
  });
}

/** 記録の削除（DELETE /records/{id}。編集者以上。要件12-4） */
export function useDeleteRecord(householdId: string, plantId: string, recordId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.DELETE('/records/{recordId}', { params: { path: { recordId } } })),
    onSuccess: () => invalidateRecordChange(queryClient, householdId, plantId),
  });
}
