import { keepPreviousData, useQuery } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import type { components } from '@/api/schema';

export type Species = components['schemas']['Species'];

/** 植物マスタの検索（GET /species。最大20件。q が空なら和名の順） */
export function useSpeciesSearch(q: string) {
  return useQuery({
    queryKey: ['species', { q }],
    placeholderData: keepPreviousData,
    // 植物マスタは全ユーザー共通で、めったに変わらない
    staleTime: 60 * 60 * 1000,
    queryFn: async () => (await unwrap(api.GET('/species', { params: { query: q === '' ? {} : { q } } }))).species,
  });
}
