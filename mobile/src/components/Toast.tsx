import { createContext, use, useEffect, useRef, useState, type ReactNode } from 'react';
import { AccessibilityInfo, Animated, StyleSheet } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

const visibleMs = 3000;

export type ToastContextValue = {
  /** 操作の結果を、画面の下に数秒だけ出す（「保存しました」など） */
  show: (message: string) => void;
};

const ToastContext = createContext<ToastContextValue | null>(null);

/**
 * トースト（アプリの層。foundations.md の 5章）。操作の結果を画面の下に数秒だけ出す。
 * 押す操作はない（取り消しのボタンつきは、TODAY-02 のスナックバー）。画面の読み上げにも伝える
 */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [message, setMessage] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => () => clearTimeout(timer.current), []);

  const show = (next: string) => {
    clearTimeout(timer.current);
    setMessage(next);
    AccessibilityInfo.announceForAccessibility(next);
    timer.current = setTimeout(() => setMessage(null), visibleMs);
  };

  return (
    <ToastContext value={{ show }}>
      {children}
      {message !== null && <ToastView message={message} />}
    </ToastContext>
  );
}

function ToastView({ message }: { message: string }) {
  const theme = useTheme();
  const insets = useSafeAreaInsets();
  // 表示のたびに作り直さない値（描画の中で ref を読まないように、state で持つ）
  const [opacity] = useState(() => new Animated.Value(0));

  useEffect(() => {
    Animated.timing(opacity, { toValue: 1, duration: 150, useNativeDriver: true }).start();
  }, [opacity]);

  return (
    <Animated.View
      pointerEvents="none"
      style={[
        styles.container,
        {
          opacity,
          // タブバーの上に出す
          bottom: insets.bottom + theme.space['2xl'] * 2,
          marginHorizontal: theme.space.lg,
          paddingHorizontal: theme.space.lg,
          paddingVertical: theme.space.md,
          borderRadius: theme.radius.md,
          backgroundColor: theme.color.text.primary,
        },
      ]}
    >
      {/* 地と文字の色を反転し、画面の内容と区別する */}
      <AppText variant="callout" style={{ color: theme.color.bg.base }}>
        {message}
      </AppText>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  container: { position: 'absolute', left: 0, right: 0, alignItems: 'center' },
});

export function useToast(): ToastContextValue {
  const value = use(ToastContext);
  if (!value) throw new Error('ToastProvider の中で使う');
  return value;
}
