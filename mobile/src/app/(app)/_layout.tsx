import { Redirect, router, Stack, usePathname, useSegments } from 'expo-router';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

import { useAuth, useProfileSetup } from '@/auth/AuthProvider';
import { useMe } from '@/features/account/useMe';
import { SelectedHouseholdProvider, useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { rememberPendingInvite } from '@/features/invitation/pendingInvite';
import { RealtimeProvider } from '@/realtime/RealtimeProvider';

// サインインしてから使う画面。未サインインなら、サインインの画面へ移す
export default function AppLayout() {
  const auth = useAuth();
  const pathname = usePathname();

  // 状態の確認中も Stack は描く（何も描かないと、開いた URL の画面が失われて先頭の画面になる）。
  // スマホでは、確認中はスプラッシュが画面を隠している
  if (auth.status === 'signedOut') {
    // 招待リンクから開いた場合は、サインインのあとに同じ参加の確認へ戻す（要件3-4）
    const invite = /^\/invite\/([^/]+)$/.exec(pathname);
    if (invite?.[1]) rememberPendingInvite(invite[1]);
    return <Redirect href="/sign-in" />;
  }

  // サインアウトするとこのレイアウトごと外れるので、選択中の世帯の状態も残らない
  return (
    <SelectedHouseholdProvider>
      {/* リアルタイム同期（18.1）。所属するすべての世帯を購読し、ほかのメンバーの変更を取り込む */}
      <RealtimeProvider>
        <AppStack />
      </RealtimeProvider>
    </SelectedHouseholdProvider>
  );
}

// 世帯がなくても開ける画面（はじめての選択・作成・参加）。segments[1] で判定する
const withoutHouseholdRoutes = new Set(['onboarding', 'households', 'join', 'invite', 'display-name', 'choice']);

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
    // タブの上に重ねる画面の戻るボタンは、矢印だけにする（前の画面の名前が、タブのグループ名「(tabs)」になるため）
    <Stack screenOptions={{ headerBackButtonDisplayMode: 'minimal' }}>
      <Stack.Screen name="index" options={{ headerShown: false }} />
      <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
      <Stack.Screen name="plants/[id]/index" options={{ title: t('plantDetail.title') }} />
      {/* PLANT-02 の登録・編集と、その上に重ねる種類の検索（PLANT-03） */}
      <Stack.Screen name="plants/new" options={{ title: t('plants.create.title'), presentation: 'modal' }} />
      <Stack.Screen name="plants/[id]/edit" options={{ title: t('plants.edit.title'), presentation: 'modal' }} />
      <Stack.Screen name="species" options={{ title: t('species.title'), presentation: 'modal' }} />
      {/* TASK-01 お手入れタスクの作成・編集 */}
      <Stack.Screen name="plants/[id]/tasks/new" options={{ title: t('tasks.create.title'), presentation: 'modal' }} />
      <Stack.Screen
        name="plants/[id]/tasks/[taskId]"
        options={{ title: t('tasks.edit.title'), presentation: 'modal' }}
      />
      {/* TODAY-03 スキップ・TODAY-04 実施日を変えて記録。今日の一覧のカードの長押しのメニューから開く OS のシート */}
      <Stack.Screen
        name="today/[taskId]/skip"
        options={{
          title: t('today.skip.title'),
          presentation: 'formSheet',
          sheetAllowedDetents: [0.75, 1],
          sheetGrabberVisible: true,
        }}
      />
      <Stack.Screen
        name="today/[taskId]/record"
        options={{
          title: t('today.record.title'),
          presentation: 'formSheet',
          sheetAllowedDetents: [0.4, 1],
          sheetGrabberVisible: true,
        }}
      />
      {/* PLANT-14 メモの作成・編集 */}
      <Stack.Screen name="plants/[id]/notes/new" options={{ title: t('notes.create.title'), presentation: 'modal' }} />
      <Stack.Screen
        name="plants/[id]/notes/[noteId]"
        options={{ title: t('notes.edit.title'), presentation: 'modal' }}
      />
      {/* PLANT-12 記録の編集・削除。履歴の記録を押すと開く OS のシート */}
      <Stack.Screen
        name="plants/[id]/records/[recordId]"
        options={{
          title: t('history.edit.title'),
          presentation: 'formSheet',
          sheetAllowedDetents: [0.5, 1],
          sheetGrabberVisible: true,
        }}
      />
      {/* PLANT-04 植え替えの提案。登録のモーダルを置き換えて出す。答えずに閉じても、提案は残る */}
      <Stack.Screen
        name="plants/[id]/repotting"
        options={{ title: t('suggestion.repotting.screenTitle'), presentation: 'modal' }}
      />
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
      {/* 題名は、開いた欄の見出しを画面の中で付ける */}
      <Stack.Screen name="choice" options={{ presentation: 'modal' }} />
      {/* LOC-02 置き場所の作成・編集。選択肢のシートを上に重ねるので、タブの外に置く */}
      <Stack.Screen name="locations/new" options={{ title: t('location.create.title'), presentation: 'modal' }} />
      <Stack.Screen name="locations/[id]" options={{ title: t('location.edit.title'), presentation: 'modal' }} />
      <Stack.Screen name="join/index" options={{ title: t('join.title') }} />
      {/* カメラは画面全体に出す */}
      <Stack.Screen name="join/scan" options={{ title: t('scan.title'), presentation: 'fullScreenModal' }} />
    </Stack>
  );
}
