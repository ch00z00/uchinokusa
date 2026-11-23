import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import type { components } from '@/api/schema';
import { plantKeys } from '@/features/plant/usePlants';

export const taskKeys = {
  /** 植物のタスク（植物の詳細のキーの下に置き、植物を取り直すときにいっしょに取り直す） */
  list: (plantId: string) => [...plantKeys.detail(plantId), 'tasks'] as const,
  defaults: (plantId: string) => [...plantKeys.detail(plantId), 'task-defaults'] as const,
};

/** 植物のお手入れタスク（GET /plants/{id}/tasks。一時停止中を含み、作成の古い順。閲覧者も見られる） */
export function useCareTasks(plantId: string) {
  return useQuery({
    queryKey: taskKeys.list(plantId),
    queryFn: async () => (await unwrap(api.GET('/plants/{plantId}/tasks', { params: { path: { plantId } } }))).tasks,
  });
}

/** タスクの作成の初期値（GET /plants/{id}/task-defaults。植物マスタのおすすめ。要件6-4） */
export function useTaskDefaults(plantId: string) {
  return useQuery({
    queryKey: taskKeys.defaults(plantId),
    queryFn: async () =>
      (await unwrap(api.GET('/plants/{plantId}/task-defaults', { params: { path: { plantId } } }))).defaults,
  });
}

/**
 * タスクの作成（POST。編集者以上）。植物の次の予定日・一覧・今日のタスク・提案（植え替えの助言）が変わるので、
 * 植物と世帯の取得をまとめて取り直す
 */
export function useCreateCareTask(householdId: string, plantId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: components['schemas']['CreateCareTaskRequest']) =>
      unwrap(api.POST('/plants/{plantId}/tasks', { params: { path: { plantId } }, body })),
    onSuccess: () => invalidateTaskChange(queryClient, householdId, plantId),
  });
}

/** タスクが変わったら、植物（タスクの一覧を含む）と世帯（植物の一覧の次の予定日・今日やること・提案）を取り直す */
const invalidateTaskChange = (queryClient: QueryClient, householdId: string, plantId: string) =>
  Promise.all([
    queryClient.invalidateQueries({ queryKey: plantKeys.detail(plantId) }),
    queryClient.invalidateQueries({ queryKey: ['households', householdId] }),
  ]);

/** タスクの変更（PATCH /tasks/{id}。編集者以上。要件8-8） */
export function useUpdateCareTask(householdId: string, plantId: string, taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: components['schemas']['UpdateCareTaskRequest']) =>
      unwrap(api.PATCH('/tasks/{taskId}', { params: { path: { taskId } }, body })),
    onSuccess: () => invalidateTaskChange(queryClient, householdId, plantId),
  });
}

/** タスクの一時停止（POST /tasks/{id}/pause。編集者以上。要件8-9） */
export function usePauseCareTask(householdId: string, plantId: string, taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST('/tasks/{taskId}/pause', { params: { path: { taskId } } })),
    onSuccess: () => invalidateTaskChange(queryClient, householdId, plantId),
  });
}

/** タスクの再開（POST /tasks/{id}/resume。編集者以上。次の予定日が過ぎていたら、サーバーが今日にする） */
export function useResumeCareTask(householdId: string, plantId: string, taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST('/tasks/{taskId}/resume', { params: { path: { taskId } } })),
    onSuccess: () => invalidateTaskChange(queryClient, householdId, plantId),
  });
}

/** タスクの削除（DELETE /tasks/{id}。編集者以上。これまでの記録は履歴に残る。要件8-10） */
export function useDeleteCareTask(householdId: string, plantId: string, taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.DELETE('/tasks/{taskId}', { params: { path: { taskId } } })),
    onSuccess: () => invalidateTaskChange(queryClient, householdId, plantId),
  });
}
