import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import { plantKeys } from '@/features/plant/usePlants';

import type { NotePage } from './note';

export const noteKeys = {
  /** 植物のメモ。植物の詳細のキーの下に置く（カレンダーのメモの日も、植物を取り直すときにいっしょに取り直す） */
  list: (plantId: string) => [...plantKeys.detail(plantId), 'notes'] as const,
};

const pageSize = 30;

/** 植物のメモ（GET /plants/{id}/notes。作成の新しい順、30件ずつ。閲覧者も見られる。要件13-2） */
export function useNotes(plantId: string) {
  return useInfiniteQuery({
    queryKey: noteKeys.list(plantId),
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }): Promise<NotePage> =>
      unwrap(
        api.GET('/plants/{plantId}/notes', {
          params: { path: { plantId }, query: { cursor: pageParam, limit: pageSize } },
        }),
      ),
    getNextPageParam: (last) => last.nextCursor,
  });
}

/**
 * メモが変わったら、植物の詳細（メモの一覧・カレンダー）を取り直す。
 * 今日やることや植物の一覧には出ないので、世帯は取り直さない
 */
function useInvalidatePlant(plantId: string) {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: plantKeys.detail(plantId) });
}

/** メモの作成（POST /plants/{id}/notes。編集者以上。要件13-1） */
export function useCreateNote(plantId: string) {
  const invalidate = useInvalidatePlant(plantId);
  return useMutation({
    mutationFn: (body: string) =>
      unwrap(api.POST('/plants/{plantId}/notes', { params: { path: { plantId } }, body: { body } })),
    onSuccess: invalidate,
  });
}

/** メモの変更（PATCH /notes/{id}。書いた本人とオーナー。要件13-3） */
export function useUpdateNote(plantId: string, noteId: string) {
  const invalidate = useInvalidatePlant(plantId);
  return useMutation({
    mutationFn: (body: string) =>
      unwrap(api.PATCH('/notes/{noteId}', { params: { path: { noteId } }, body: { body } })),
    onSuccess: invalidate,
  });
}

/** メモの削除（DELETE /notes/{id}。書いた本人とオーナー。要件13-3） */
export function useDeleteNote(plantId: string, noteId: string) {
  const invalidate = useInvalidatePlant(plantId);
  return useMutation({
    mutationFn: () => unwrap(api.DELETE('/notes/{noteId}', { params: { path: { noteId } } })),
    onSuccess: invalidate,
  });
}
