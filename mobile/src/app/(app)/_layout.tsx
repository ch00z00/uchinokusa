import { Redirect, router, Stack, useSegments } from 'expo-router';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

import { useAuth, useProfileSetup } from '@/auth/AuthProvider';
import { useMe } from '@/features/account/useMe';

// サインインしてから使う画面。未サインインなら、サインインの画面へ移す
export default function AppLayout() {
  const auth = useAuth();
  const { t } = useTranslation();
  const segments = useSegments();
  const me = useMe();
  const { needsProfileSetup } = useProfileSetup();

  // 初めてのサインインのあと、または表示名が空（Apple は名前をくれないことがある）なら、表示名の設定へ
  const needsDisplayName = needsProfileSetup || me.data?.displayName === '';
  const onDisplayName = segments.at(-1) === 'display-name';
  useEffect(() => {
    if (auth.status === 'signedIn' && needsDisplayName && !onDisplayName) router.replace('/display-name');
  }, [auth.status, needsDisplayName, onDisplayName]);

  // 状態の確認中も Stack は描く（何も描かないと、開いた URL の画面が失われて先頭の画面になる）。
  // スマホでは、確認中はスプラッシュが画面を隠している
  // TODO(15.2)：招待リンクから開いた場合は、サインインのあとに同じ画面へ戻す
  if (auth.status === 'signedOut') return <Redirect href="/sign-in" />;

  return (
    <Stack>
      <Stack.Screen name="index" options={{ headerShown: false }} />
      <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
      <Stack.Screen name="plants/[id]" options={{ title: t('plantDetail.title') }} />
      <Stack.Screen name="settings/index" options={{ title: t('settings.title') }} />
      <Stack.Screen name="invite/[token]" options={{ title: t('invite.title') }} />
      {/* 表示名を決めるまでは戻れないようにする */}
      <Stack.Screen
        name="display-name"
        options={{ title: t('displayName.title'), headerBackVisible: false, gestureEnabled: false }}
      />
    </Stack>
  );
}
