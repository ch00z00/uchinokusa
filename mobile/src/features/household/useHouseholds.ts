import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import type { components } from '@/api/schema';
import { useAuth } from '@/auth/AuthProvider';

export type Household = components['schemas']['Household'];
export type Member = components['schemas']['Member'];

export const householdKeys = {
  all: ['households'] as const,
  members: (householdId: string) => ['households', householdId, 'members'] as const,
};

/** 所属する世帯（GET /households。作成の古い順） */
export function useHouseholds() {
  const auth = useAuth();
  return useQuery({
    queryKey: householdKeys.all,
    queryFn: async () => (await unwrap(api.GET('/households'))).households,
    enabled: auth.status === 'signedIn',
  });
}

/** 世帯の作成（POST /households）。作成したユーザーはオーナーになる */
export function useCreateHousehold() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: components['schemas']['CreateHouseholdRequest']) => unwrap(api.POST('/households', { body })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: householdKeys.all }),
  });
}

/** 世帯名・地域の変更（PATCH /households/{id}。オーナーのみ） */
export function useUpdateHousehold(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: components['schemas']['UpdateHouseholdRequest']) =>
      unwrap(api.PATCH('/households/{householdId}', { params: { path: { householdId } }, body })),
    onSuccess: (updated) =>
      queryClient.setQueryData<Household[]>(householdKeys.all, (list) =>
        list?.map((h) => (h.id === updated.id ? updated : h)),
      ),
  });
}

/** 世帯の削除（DELETE /households/{id}。オーナーのみ。配下のデータもすべて消える） */
export function useDeleteHousehold(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.DELETE('/households/{householdId}', { params: { path: { householdId } } })),
    onSuccess: () => forgetHousehold(queryClient, householdId),
  });
}

/** 世帯のメンバー（GET /households/{id}/members。参加の古い順）。householdId が空なら取りに行かない */
export function useMembers(householdId: string) {
  return useQuery({
    queryKey: householdKeys.members(householdId),
    enabled: householdId !== '',
    queryFn: () => fetchMembers(householdId),
  });
}

/** 世帯のメンバーを取る（リアルタイム同期で、操作した人の名前を引くときにも使う） */
export const fetchMembers = async (householdId: string) =>
  (await unwrap(api.GET('/households/{householdId}/members', { params: { path: { householdId } } }))).members;

/** ロールの変更（編集者 ⇄ 閲覧者。オーナーのみ） */
export function useUpdateMemberRole(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: 'editor' | 'viewer' }) =>
      unwrap(
        api.PATCH('/households/{householdId}/members/{userId}', {
          params: { path: { householdId, userId } },
          body: { role },
        }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: householdKeys.members(householdId) }),
  });
}

/** メンバーを外す（オーナーのみ）。userId が自分なら退出 */
export function useRemoveMember(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ userId }: { userId: string; self: boolean }) =>
      unwrap(api.DELETE('/households/{householdId}/members/{userId}', { params: { path: { householdId, userId } } })),
    onSuccess: (_, { self }) =>
      self
        ? forgetHousehold(queryClient, householdId)
        : queryClient.invalidateQueries({ queryKey: householdKeys.members(householdId) }),
  });
}

/** オーナー権限の移譲。移した先がオーナーになり、自分は編集者になる（要件4-8） */
export function useTransferOwnership(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) =>
      unwrap(
        api.POST('/households/{householdId}/ownership-transfer', {
          params: { path: { householdId } },
          body: { userId },
        }),
      ),
    // 自分のロール（世帯の一覧）と、メンバーのロールの両方が変わる
    onSuccess: () => queryClient.invalidateQueries({ queryKey: householdKeys.all }),
  });
}

/** 世帯から抜けた（退出・削除）：その世帯のキャッシュを消し、世帯の一覧を取り直す（選択中の世帯は別の世帯に移る） */
async function forgetHousehold(queryClient: ReturnType<typeof useQueryClient>, householdId: string) {
  queryClient.removeQueries({ queryKey: ['households', householdId] });
  await queryClient.invalidateQueries({ queryKey: householdKeys.all, exact: true });
}
