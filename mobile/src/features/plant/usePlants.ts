import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import type { components } from '@/api/schema';

import type { Plant } from './plantList';

export const plantKeys = {
  /** 世帯の植物にかかわる取得すべて（置き場所の変更・削除のあとに取り直す） */
  all: (householdId: string) => ['households', householdId, 'plants'] as const,
  list: (householdId: string, q: string) => ['households', householdId, 'plants', 'list', { q }] as const,
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
