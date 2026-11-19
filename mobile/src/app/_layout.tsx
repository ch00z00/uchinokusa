import '@/i18n';

import { QueryClientProvider } from '@tanstack/react-query';
import { Stack, ThemeProvider } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { useEffect, useState } from 'react';

import { AuthProvider, useAuth } from '@/auth/AuthProvider';
import { ToastProvider } from '@/components/Toast';
import { createQueryClient } from '@/query/queryClient';
import { useAppStateFocus } from '@/query/useAppStateFocus';
import { toNavigationTheme, useTheme } from '@/theme';

// サインインの状態がわかるまで、起動の画面（スプラッシュ）を出したままにする
void SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const theme = useTheme();
  const [queryClient] = useState(createQueryClient);
  useAppStateFocus();

  return (
    <ThemeProvider value={toNavigationTheme(theme)}>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <ToastProvider>
            <SplashScreenController />
            {/* サインインの前後の振り分けは、(app) と (auth) のレイアウトで行う */}
            <Stack screenOptions={{ headerShown: false }} />
          </ToastProvider>
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
