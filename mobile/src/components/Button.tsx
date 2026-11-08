import { ActivityIndicator, Pressable, StyleSheet, type PressableProps } from 'react-native';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

export type ButtonProps = Omit<PressableProps, 'children' | 'style'> & {
  title: string;
  /** primary：アクセントの背景（主な操作）。secondary：枠だけ */
  variant?: 'primary' | 'secondary';
  loading?: boolean;
};

/**
 * アプリの層のボタン（foundations.md の 5章）。
 * 削除などの危険な操作には使わない（OS の確認のアラートに任せる）
 */
export function Button({ title, variant = 'primary', loading = false, disabled, ...props }: ButtonProps) {
  const theme = useTheme();
  const primary = variant === 'primary';
  const isDisabled = disabled || loading;

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: isDisabled, busy: loading }}
      disabled={isDisabled}
      style={({ pressed }) => [
        styles.base,
        {
          minHeight: theme.size.tapMin,
          paddingHorizontal: theme.space.lg,
          borderRadius: theme.radius.md,
          borderWidth: theme.borderWidth.hairline,
          borderColor: theme.color.accent.default,
          backgroundColor: primary ? theme.color.accent.default : 'transparent',
        },
        // 押している間と無効のときは、色を足さずに薄くする（新しい色のトークンを増やさない）
        (pressed || isDisabled) && styles.dimmed,
      ]}
      {...props}
    >
      {loading ? (
        <ActivityIndicator color={primary ? theme.color.onAccent : theme.color.accent.default} />
      ) : (
        <AppText variant="headline" color={primary ? 'onAccent' : 'accent'}>
          {title}
        </AppText>
      )}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  base: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  dimmed: {
    opacity: 0.6,
  },
});
