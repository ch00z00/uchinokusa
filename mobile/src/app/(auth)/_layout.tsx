import { Redirect, Stack } from 'expo-router';

import { useAuth } from '@/auth/AuthProvider';
import { homeHref } from '@/features/invitation/pendingInvite';

// サインインの前の画面。サインインしたら「今日」へ移す（招待リンクから来た場合は、その参加の確認へ）
export default function AuthLayout() {
  const auth = useAuth();

  if (auth.status === 'signedIn') return <Redirect href={homeHref()} />;

  return <Stack screenOptions={{ headerShown: false }} />;
}
