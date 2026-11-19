import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import type { components } from '@/api/schema';
import { householdKeys } from '@/features/household/useHouseholds';

export type Invitation = components['schemas']['Invitation'];
export type CreatedInvitation = components['schemas']['CreatedInvitation'];
export type InvitationPreview = components['schemas']['InvitationPreview'];
/** 招待で付与できるロール（オーナーは招待できない） */
export type InvitableRole = 'editor' | 'viewer';

export const invitationKeys = {
  list: (householdId: string) => ['households', householdId, 'invitations'] as const,
  preview: (tokenOrCode: string) => ['invitations', tokenOrCode] as const,
};

/** 有効な招待の一覧（GET /households/{id}/invitations。オーナーのみ。新しい順） */
export function useInvitations(householdId: string) {
  return useQuery({
    queryKey: invitationKeys.list(householdId),
    queryFn: async () =>
      (await unwrap(api.GET('/households/{householdId}/invitations', { params: { path: { householdId } } })))
        .invitations,
  });
}

/** 招待を作る（POST /households/{id}/invitations）。招待リンクの URL は、この結果でしか受け取れない */
export function useCreateInvitation(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (role: InvitableRole) =>
      unwrap(api.POST('/households/{householdId}/invitations', { params: { path: { householdId } }, body: { role } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: invitationKeys.list(householdId) }),
  });
}

/** 招待を取り消す（要件3-7）。リンク・QR・コードは、ただちに使えなくなる */
export function useRevokeInvitation(householdId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (invitationId: string) =>
      unwrap(
        api.DELETE('/households/{householdId}/invitations/{invitationId}', {
          params: { path: { householdId, invitationId } },
        }),
      ),
    onSuccess: (_, invitationId) =>
      queryClient.setQueryData<Invitation[]>(invitationKeys.list(householdId), (list) =>
        list?.filter((i) => i.id !== invitationId),
      ),
  });
}

/** 招待の内容（世帯名と付与されるロール。INV-12）。トークンでもコードでも引ける */
export function useInvitationPreview(tokenOrCode: string) {
  return useQuery({
    queryKey: invitationKeys.preview(tokenOrCode),
    queryFn: () => unwrap(api.GET('/invitations/{tokenOrCode}', { params: { path: { tokenOrCode } } })),
  });
}

/** 招待を受けて参加する（要件3-3）。参加した世帯を返す */
export function useAcceptInvitation(tokenOrCode: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST('/invitations/{tokenOrCode}/accept', { params: { path: { tokenOrCode } } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: householdKeys.all, exact: true }),
  });
}
