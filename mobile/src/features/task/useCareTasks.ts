import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

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
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: plantKeys.detail(plantId) }),
        queryClient.invalidateQueries({ queryKey: ['households', householdId] }),
      ]),
  });
}
