import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api, unwrap } from '@/api';
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
        clearStorage: clearAppStorage,
      }),
  });
}
