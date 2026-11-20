import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import type { components } from '@/api/schema';

import { sortPlantsByArchivedAt, type Plant } from './plantList';

export const plantKeys = {
  /** 世帯の植物にかかわる取得すべて（置き場所の変更・削除のあとに取り直す） */
  all: (householdId: string) => ['households', householdId, 'plants'] as const,
  list: (householdId: string, q: string) => ['households', householdId, 'plants', 'list', { q }] as const,
  archived: (householdId: string) => ['households', householdId, 'plants', 'archived'] as const,
  /** 植物の詳細すべて */
  details: ['plants'] as const,
  detail: (plantId: string) => ['plants', plantId] as const,
};

/**
 * 世帯の植物（GET /households/{id}/plants。アーカイブしたものを除く）。q は名前の検索（空なら全件）。
 * 検索語を変えている間は、前の結果を出したままにする（一覧がちらつかないように）
 */
export function usePlants(householdId: string, q: string) {
  return useQuery({
    queryKey: plantKeys.list(householdId, q),
    enabled: householdId !== '',
    placeholderData: keepPreviousData,
    queryFn: async () =>
      (
        await unwrap(
          api.GET('/households/{householdId}/plants', {
            params: { path: { householdId }, query: q === '' ? {} : { q } },
          }),
        )
      ).plants,
  });
}

/** アーカイブした植物（GET /households/{id}/plants?archived=true。PLANT-05）。アーカイブの新しい順 */
export function useArchivedPlants(householdId: string) {
  return useQuery({
    queryKey: plantKeys.archived(householdId),
    enabled: householdId !== '',
    queryFn: async () =>
      sortPlantsByArchivedAt(
        (
          await unwrap(
            api.GET('/households/{householdId}/plants', {
              params: { path: { householdId }, query: { archived: true } },
            }),
          )
        ).plants,
      ),
  });
}

/** 植物の詳細（GET /plants/{id}。閲覧者も、アーカイブした植物も見られる） */
export function usePlant(plantId: string) {
  return useQuery({
    queryKey: plantKeys.detail(plantId),
    queryFn: () => unwrap(api.GET('/plants/{plantId}', { params: { path: { plantId } } })),
  });
}

/** 植物の登録（POST。編集者以上） */
export function useCreatePlant(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: components['schemas']['CreatePlantRequest']) =>
      unwrap(api.POST('/households/{householdId}/plants', { params: { path: { householdId } }, body })),
    onSuccess: (created) => {
      queryClient.setQueryData<Plant>(plantKeys.detail(created.id), created);
      return queryClient.invalidateQueries({ queryKey: plantKeys.all(householdId) });
    },
  });
}

/** 植物の情報の変更（PATCH。編集者以上） */
export function useUpdatePlant(householdId: string, plantId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: components['schemas']['UpdatePlantRequest']) =>
      unwrap(api.PATCH('/plants/{plantId}', { params: { path: { plantId } }, body })),
    onSuccess: (updated) => {
      queryClient.setQueryData<Plant>(plantKeys.detail(plantId), updated);
      return queryClient.invalidateQueries({ queryKey: plantKeys.all(householdId) });
    },
  });
}

/**
 * 世帯の取得すべてを取り直す。アーカイブ・削除で、一覧のほかに、提案と今日のタスクも変わるため
 * （提案のキーは useSuggestions.ts にあり、そちらがこのファイルを読むので、世帯の前方一致でまとめて取り直す）
 */
const invalidateHousehold = (queryClient: ReturnType<typeof useQueryClient>, householdId: string) =>
  queryClient.invalidateQueries({ queryKey: ['households', householdId] });

/** アーカイブ（POST /plants/{id}/archive。編集者以上）。一覧とリマインダーから外れ、記録とメモは残る（要件5-8） */
export function useArchivePlant(householdId: string, plantId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST('/plants/{plantId}/archive', { params: { path: { plantId } } })),
    onSuccess: (archived) => {
      queryClient.setQueryData<Plant>(plantKeys.detail(plantId), archived);
      return invalidateHousehold(queryClient, householdId);
    },
  });
}

/** アーカイブの解除（POST /plants/{id}/unarchive。編集者以上） */
export function useUnarchivePlant(householdId: string, plantId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST('/plants/{plantId}/unarchive', { params: { path: { plantId } } })),
    onSuccess: (restored) => {
      queryClient.setQueryData<Plant>(plantKeys.detail(plantId), restored);
      return invalidateHousehold(queryClient, householdId);
    },
  });
}

/** 削除（DELETE /plants/{id}。編集者以上）。タスク・記録・メモもいっしょに消え、元に戻せない（要件5-7） */
export function useDeletePlant(householdId: string, plantId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.DELETE('/plants/{plantId}', { params: { path: { plantId } } })),
    onSuccess: () => {
      // 閉じていく詳細の画面が、消えた植物を取り直さないように
      queryClient.removeQueries({ queryKey: plantKeys.detail(plantId) });
      return invalidateHousehold(queryClient, householdId);
    },
  });
}
