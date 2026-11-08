import { Redirect, Stack } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { useAuth } from '@/auth/AuthProvider';

// サインインしてから使う画面。未サインインなら、サインインの画面へ移す
export default function AppLayout() {
  const auth = useAuth();
  const { t } = useTranslation();

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
    </Stack>
  );
}
