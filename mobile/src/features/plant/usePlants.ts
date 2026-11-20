import { keepPreviousData, useQuery } from '@tanstack/react-query';

import { api, unwrap } from '@/api';

export const plantKeys = {
  /** 世帯の植物にかかわる取得すべて（置き場所の変更・削除のあとに取り直す） */
  all: (householdId: string) => ['households', householdId, 'plants'] as const,
  list: (householdId: string, q: string) => ['households', householdId, 'plants', 'list', { q }] as const,
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
