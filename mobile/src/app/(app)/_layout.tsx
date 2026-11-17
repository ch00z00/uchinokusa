import { Redirect, router, Stack, useSegments } from 'expo-router';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

import { useAuth, useProfileSetup } from '@/auth/AuthProvider';
import { useMe } from '@/features/account/useMe';
import { SelectedHouseholdProvider, useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';

// サインインしてから使う画面。未サインインなら、サインインの画面へ移す
export default function AppLayout() {
  const auth = useAuth();

  // 状態の確認中も Stack は描く（何も描かないと、開いた URL の画面が失われて先頭の画面になる）。
  // スマホでは、確認中はスプラッシュが画面を隠している
  // TODO(15.2)：招待リンクから開いた場合は、サインインのあとに同じ画面へ戻す
  if (auth.status === 'signedOut') return <Redirect href="/sign-in" />;

  // サインアウトするとこのレイアウトごと外れるので、選択中の世帯の状態も残らない
  return (
    <SelectedHouseholdProvider>
      <AppStack />
    </SelectedHouseholdProvider>
  );
}

// 世帯がなくても開ける画面（はじめての選択・作成・参加）。segments[1] で判定する
const withoutHouseholdRoutes = new Set(['onboarding', 'households', 'join', 'invite', 'display-name']);

function AppStack() {
  const auth = useAuth();
  const { t } = useTranslation();
  const segments = useSegments();
  const me = useMe();
  const { needsProfileSetup } = useProfileSetup();
  const { state } = useSelectedHousehold();

  // 1. 初めてのサインインのあと、または表示名が空（Apple は名前をくれないことがある）なら、表示名の設定へ
  const needsDisplayName = needsProfileSetup || me.data?.displayName === '';
  const onDisplayName = segments.at(-1) === 'display-name';
  // 2. どの世帯にも属していなければ、はじめての選択へ（要件2-2）。退出・削除で世帯がなくなったときも
  const needsOnboarding = state.status === 'none' && !withoutHouseholdRoutes.has(segments[1] ?? '');
  useEffect(() => {
    if (auth.status !== 'signedIn') return;
    if (needsDisplayName) {
      if (!onDisplayName) router.replace('/display-name');
    } else if (needsOnboarding) {
      router.replace('/onboarding');
    }
  }, [auth.status, needsDisplayName, onDisplayName, needsOnboarding]);

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
      {/* 世帯がないあいだの入口なので、戻る先がない */}
      <Stack.Screen
        name="onboarding"
        options={{ title: t('onboarding.title'), headerBackVisible: false, gestureEnabled: false }}
      />
      <Stack.Screen name="households/new" options={{ title: t('household.create'), presentation: 'modal' }} />
      {/* HH-10 は OS のシート（下から出て、半分の高さで止まる） */}
      <Stack.Screen
        name="households/switch"
        options={{
          title: t('household.switcher.title'),
          presentation: 'formSheet',
          sheetAllowedDetents: [0.5, 1],
          sheetGrabberVisible: true,
        }}
      />
      <Stack.Screen name="join" options={{ title: t('household.join') }} />
      <Stack.Screen name="household/index" options={{ title: t('household.settings.title') }} />
      <Stack.Screen name="household/members/index" options={{ title: t('household.members.title') }} />
      <Stack.Screen name="household/members/[userId]" options={{ title: '' }} />
      <Stack.Screen name="household/transfer" options={{ title: t('household.transfer.title') }} />
    </Stack>
  );
}
