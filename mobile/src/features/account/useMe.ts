import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import { useAuth } from '@/auth/AuthProvider';

export const meQueryKey = ['me'] as const;

/** 自分のプロフィール（GET /me）。サインインしていなければ取りに行かない */
export function useMe() {
  const auth = useAuth();
  return useQuery({
    queryKey: meQueryKey,
    queryFn: () => unwrap(api.GET('/me')),
    enabled: auth.status === 'signedIn',
  });
}

/** 表示名の変更（PUT /me） */
export function useUpdateDisplayName() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (displayName: string) => unwrap(api.PUT('/me', { body: { displayName } })),
    onSuccess: (me) => queryClient.setQueryData(meQueryKey, me),
  });
}
