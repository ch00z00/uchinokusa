import { Redirect, Stack } from 'expo-router';

import { useAuth } from '@/auth/AuthProvider';

// サインインの前の画面。サインインしたら「今日」へ移す
export default function AuthLayout() {
  const auth = useAuth();

  if (auth.status === 'signedIn') return <Redirect href="/today" />;

  return <Stack screenOptions={{ headerShown: false }} />;
}
