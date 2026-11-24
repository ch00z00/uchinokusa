import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import type { components } from '@/api/schema';
import { ApiError } from '@/api/errors';
import { authClient } from '@/auth/authClient';
import { performSignOut } from '@/auth/signOut';
import { clearPendingInvite } from '@/features/invitation/pendingInvite';
import { clearAppStorage } from '@/lib/deviceStorage';

export type HouseholdSummary = components['schemas']['HouseholdSummary'];

export type DeleteError =
  /** ほかにメンバーのいる世帯の唯一のオーナー。先にオーナー権限を移す（要件1-7） */
  | { kind: 'owner_transfer_required'; households: HouseholdSummary[] }
  /** Apple の認可コードがない・期限切れ。もう一度押して、再認証し直す */
  | { kind: 'apple_reauth' }
  | { kind: 'other'; error: unknown };

/** アカウントの削除（DELETE /me）の失敗を、画面の出し分けに分ける */
export function classifyDeleteError(error: unknown): DeleteError {
  if (error instanceof ApiError) {
    if (error.code === 'owner_transfer_required') {
      const body = error.body as { households?: unknown } | undefined;
      const households = Array.isArray(body?.households) ? (body.households as HouseholdSummary[]) : [];
      return { kind: 'owner_transfer_required', households };
    }
    if (error.fields.appleAuthorizationCode !== undefined) return { kind: 'apple_reauth' };
  }
  return { kind: 'other', error };
}

/** Apple の再認証を閉じた。何も知らせずにやめる */
export class DeleteCancelled extends Error {}

/**
 * アカウントの削除（DELETE /me。要件1-6・1-7。タスク 19.2）。
 * Apple でサインインした人は、削除の直前に Apple で再認証して、認可コードを送る。
 * 成功したら、この端末のサインイン情報・キャッシュ・保存した値を消す（端末の通知の登録はサーバーが消す）
 */
export function useDeleteAccount() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      let appleAuthorizationCode: string | undefined;
      if (authClient.currentProvider() === 'apple') {
        const code = await authClient.appleAuthorizationCode();
        if (code === null) throw new DeleteCancelled();
        appleAuthorizationCode = code;
      }
      await unwrap(api.DELETE('/me', { body: appleAuthorizationCode ? { appleAuthorizationCode } : {} }));
      await performSignOut({
        // 端末はサーバーが消したので、解除の API は呼ばない
        getDeviceId: async () => null,
        unregisterDevice: async () => undefined,
        // Firebase のユーザーはサーバーが消している。端末のサインインの状態だけを消す
        signOut: () => authClient.signOut(),
        clearCache: () => queryClient.clear(),
        clearStorage: () => {
          clearPendingInvite();
          return clearAppStorage();
        },
      });
    },
  });
}
