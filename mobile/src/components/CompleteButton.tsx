import { SymbolView } from 'expo-symbols';
import { Pressable } from 'react-native';

import { useTheme } from '@/theme';

type Props = {
  /** 読み上げの名前（「水やりを完了」） */
  accessibilityLabel: string;
  onPress: () => void;
  disabled?: boolean;
};

/**
 * 完了の丸いボタン（アプリの層。foundations.md の 5章の CompleteButton。TaskCard の右。2026-11-23 にユーザーが選んだ形）。
 * 36pt の円（押せる領域は 44pt 以上）に、accent の枠とチェックのシンボル。押している間は地を accent/subtle にする
 */
export function CompleteButton({ accessibilityLabel, onPress, disabled }: Props) {
  const theme = useTheme();
  // 見た目の丸は 36pt にし、押せる領域は周りの余白（hitSlop）で 44pt 以上にする（カードの文字の幅を狭めないため）
  const size = 36;
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      accessibilityState={{ disabled }}
      disabled={disabled}
      onPress={onPress}
      hitSlop={(theme.size.tapMin - size) / 2 + theme.space.xs}
      style={({ pressed }) => ({
        width: size,
        height: size,
        borderRadius: theme.radius.full,
        borderWidth: theme.borderWidth.thick,
        borderColor: theme.color.accent.default,
        backgroundColor: pressed ? theme.color.accent.subtle : 'transparent',
        alignItems: 'center',
        justifyContent: 'center',
        opacity: disabled ? 0.6 : 1,
      })}
    >
      <SymbolView
        name={{ ios: 'checkmark', android: 'check' }}
        size={theme.size.icon.md}
        tintColor={theme.color.accent.default}
        style={{ width: theme.size.icon.md, height: theme.size.icon.md }}
        accessibilityElementsHidden
        importantForAccessibility="no-hide-descendants"
      />
    </Pressable>
  );
}
