import '@/i18n';

import { QueryClientProvider } from '@tanstack/react-query';
import { DarkTheme, DefaultTheme, Stack, ThemeProvider } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { useEffect, useState } from 'react';
import { useColorScheme } from 'react-native';

import { AuthProvider, useAuth } from '@/auth/AuthProvider';
import { createQueryClient } from '@/query/queryClient';
import { useAppStateFocus } from '@/query/useAppStateFocus';

// サインインの状態がわかるまで、起動の画面（スプラッシュ）を出したままにする
void SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const colorScheme = useColorScheme();
  const [queryClient] = useState(createQueryClient);
  useAppStateFocus();

  return (
    <ThemeProvider value={colorScheme === 'dark' ? DarkTheme : DefaultTheme}>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <SplashScreenController />
          {/* サインインの前後の振り分けは、(app) と (auth) のレイアウトで行う */}
          <Stack screenOptions={{ headerShown: false }} />
        </AuthProvider>
      </QueryClientProvider>
    </ThemeProvider>
  );
}

function SplashScreenController() {
  const { status } = useAuth();
  useEffect(() => {
    if (status !== 'loading') SplashScreen.hide();
  }, [status]);
  return null;
}
