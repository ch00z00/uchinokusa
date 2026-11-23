import { createContext, use, useEffect, useRef, useState, type ReactNode } from 'react';
import { AccessibilityInfo, Animated, Pressable, StyleSheet, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

const visibleMs = 5000;

export type SnackbarOptions = {
  message: string;
  /** 右のボタンの文言（「取り消す」） */
  actionLabel: string;
  onAction: () => void;
};

type SnackbarContextValue = {
  /** 取り消しのボタンつきの知らせを、画面の下に5秒出す。前のものがあれば置き換える */
  show: (options: SnackbarOptions) => void;
  hide: () => void;
};

const SnackbarContext = createContext<SnackbarContextValue | null>(null);

/**
 * スナックバー（アプリの層。foundations.md の 5章の Snackbar。TODAY-02）。
 * 完了の直後に「水やりを記録しました　取り消す」を5秒出す。連続したときは、最後の1件だけを出す
 */
export function SnackbarProvider({ children }: { children: ReactNode }) {
  const [current, setCurrent] = useState<(SnackbarOptions & { id: number }) | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const seq = useRef(0);

  useEffect(() => () => clearTimeout(timer.current), []);

  const hide = () => {
    clearTimeout(timer.current);
    setCurrent(null);
  };
  const show = (options: SnackbarOptions) => {
    clearTimeout(timer.current);
    seq.current += 1;
    setCurrent({ ...options, id: seq.current });
    AccessibilityInfo.announceForAccessibility(options.message);
    timer.current = setTimeout(() => setCurrent(null), visibleMs);
  };

  return (
    <SnackbarContext value={{ show, hide }}>
      {children}
      {current !== null && (
        <SnackbarView
          key={current.id}
          message={current.message}
          actionLabel={current.actionLabel}
          onAction={() => {
            hide();
            current.onAction();
          }}
        />
      )}
    </SnackbarContext>
  );
}

function SnackbarView({
  message,
  actionLabel,
  onAction,
}: Omit<SnackbarOptions, 'onAction'> & { onAction: () => void }) {
  const theme = useTheme();
  const insets = useSafeAreaInsets();
  const [opacity] = useState(() => new Animated.Value(0));

  useEffect(() => {
    Animated.timing(opacity, { toValue: 1, duration: 150, useNativeDriver: true }).start();
  }, [opacity]);

  return (
    <Animated.View
      pointerEvents="box-none"
      style={[
        styles.container,
        {
          opacity,
          // Toast と同じく、タブバーの上に出す
          bottom: insets.bottom + theme.space['2xl'] * 2,
          paddingHorizontal: theme.space.lg,
        },
      ]}
    >
      <View
        style={[
          styles.bar,
          {
            gap: theme.space.md,
            paddingLeft: theme.space.lg,
            borderRadius: theme.radius.md,
            backgroundColor: theme.color.text.primary,
          },
        ]}
      >
        {/* 地と文字の色を反転し、画面の内容と区別する（Toast と同じ） */}
        <AppText variant="callout" style={[styles.message, { color: theme.color.bg.base }]}>
          {message}
        </AppText>
        <Pressable
          accessibilityRole="button"
          onPress={onAction}
          style={{ minHeight: theme.size.tapMin, justifyContent: 'center', paddingHorizontal: theme.space.lg }}
        >
          <AppText variant="headline" style={{ color: theme.color.accent.subtle }}>
            {actionLabel}
          </AppText>
        </Pressable>
      </View>
    </Animated.View>
  );
}

export function useSnackbar(): SnackbarContextValue {
  const value = use(SnackbarContext);
  if (!value) throw new Error('useSnackbar は SnackbarProvider の中で使う');
  return value;
}

const styles = StyleSheet.create({
  container: { position: 'absolute', left: 0, right: 0 },
  bar: { flexDirection: 'row', alignItems: 'center' },
  message: { flex: 1 },
});
