import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
import { clearPendingInvite } from '@/features/invitation/pendingInvite';
import { clearAppStorage, getDeviceId } from '@/lib/deviceStorage';

import { authClient } from './authClient';
import { performSignOut } from './signOut';

export function useSignOut() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      performSignOut({
        getDeviceId,
        unregisterDevice: (deviceId) =>
          unwrap(api.DELETE('/me/devices/{deviceId}', { params: { path: { deviceId } } })),
        signOut: () => authClient.signOut(),
        clearCache: () => queryClient.clear(),
        // 開いていた招待も忘れる（次にサインインした人を、その招待へ進めない）
        clearStorage: () => {
          clearPendingInvite();
          return clearAppStorage();
        },
      }),
  });
}
