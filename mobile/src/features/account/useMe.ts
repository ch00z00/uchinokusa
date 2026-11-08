import { useQuery } from '@tanstack/react-query';

import { api, unwrap } from '@/api';

export const meQueryKey = ['me'] as const;

/** 自分のプロフィール（GET /me） */
export function useMe() {
  return useQuery({
    queryKey: meQueryKey,
    queryFn: () => unwrap(api.GET('/me')),
  });
}
