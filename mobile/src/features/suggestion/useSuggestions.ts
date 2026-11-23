import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import { plantKeys } from '@/features/plant/usePlants';
import { toDayString } from '@/lib/formatDate';

export const suggestionKeys = {
  list: (householdId: string) => ['households', householdId, 'suggestions'] as const,
};

/** 世帯の有効な提案を取る（GET /households/{id}/suggestions。端末での今日に有効なもの） */
export const fetchSuggestions = async (householdId: string) =>
  (
    await unwrap(
      api.GET('/households/{householdId}/suggestions', {
        params: { path: { householdId }, query: { date: toDayString(new Date()) } },
      }),
    )
  ).suggestions;

/** 世帯の有効な提案 */
export function useSuggestions(householdId: string) {
  return useQuery({
    queryKey: suggestionKeys.list(householdId),
    queryFn: () => fetchSuggestions(householdId),
  });
}

/** 提案の採用（編集者以上）。植え替えの提案なら、提案日（date を送ればその日）のタスクができる */
export function useAcceptSuggestion(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ suggestionId, date }: { suggestionId: string; date?: string }) =>
      unwrap(
        api.POST('/suggestions/{suggestionId}/accept', {
          params: { path: { suggestionId } },
          body: date ? { date } : {},
        }),
      ),
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: suggestionKeys.list(householdId) }),
        // タスクができて、植物の次の予定日が変わる
        queryClient.invalidateQueries({ queryKey: plantKeys.all(householdId) }),
        queryClient.invalidateQueries({ queryKey: plantKeys.details }),
      ]),
  });
}

/** 提案の見送り（編集者以上）。同じ提案は、このあと出ない */
export function useDismissSuggestion(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (suggestionId: string) =>
      unwrap(api.POST('/suggestions/{suggestionId}/dismiss', { params: { path: { suggestionId } } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: suggestionKeys.list(householdId) }),
  });
}
