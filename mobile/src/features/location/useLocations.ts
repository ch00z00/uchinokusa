import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import type { components } from '@/api/schema';

import { plantKeys } from '@/features/plant/usePlants';

import type { Location } from './locationValidation';

export const locationKeys = {
  list: (householdId: string) => ['households', householdId, 'locations'] as const,
};

/** 世帯の置き場所（GET /households/{id}/locations。作成の古い順。閲覧者も見られる） */
export function useLocations(householdId: string) {
  return useQuery({
    queryKey: locationKeys.list(householdId),
    enabled: householdId !== '',
    queryFn: async () =>
      (await unwrap(api.GET('/households/{householdId}/locations', { params: { path: { householdId } } }))).locations,
  });
}

/** 置き場所の作成（POST。編集者以上） */
export function useCreateLocation(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: components['schemas']['CreateLocationRequest']) =>
      unwrap(api.POST('/households/{householdId}/locations', { params: { path: { householdId } }, body })),
    onSuccess: (created) =>
      queryClient.setQueryData<Location[]>(locationKeys.list(householdId), (list) => list && [...list, created]),
  });
}

/** 置き場所の変更（PATCH。編集者以上）。紐づく植物の表示（置き場所名）は、置き場所の一覧から引くので取り直さなくてよい */
export function useUpdateLocation(householdId: string, locationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: components['schemas']['UpdateLocationRequest']) =>
      unwrap(api.PATCH('/locations/{locationId}', { params: { path: { locationId } }, body })),
    onSuccess: (updated) =>
      queryClient.setQueryData<Location[]>(locationKeys.list(householdId), (list) =>
        list?.map((l) => (l.id === updated.id ? updated : l)),
      ),
  });
}

/** 置き場所の削除（DELETE。編集者以上）。紐づく植物は「置き場所未設定」になる */
export function useDeleteLocation(householdId: string, locationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.DELETE('/locations/{locationId}', { params: { path: { locationId } } })),
    onSuccess: () => {
      queryClient.setQueryData<Location[]>(locationKeys.list(householdId), (list) =>
        list?.filter((l) => l.id !== locationId),
      );
      // 紐づいていた植物は「置き場所未設定」になるので、植物の一覧も取り直す
      return queryClient.invalidateQueries({ queryKey: plantKeys.all(householdId) });
    },
  });
}
