import { SymbolView } from 'expo-symbols';
import { View } from 'react-native';

import { useTheme } from '@/theme';

/**
 * 選択モードの、選んだかの印（アプリの層。foundations.md の 5章の SelectionCheck。TaskCard の右）。
 * 押す操作はカードが受け持ち、読み上げもカードの選択の状態で伝える（ここは読み上げない）
 */
export function SelectionCheck({ selected }: { selected: boolean }) {
  const theme = useTheme();
  const size = 24;
  return (
    <View
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      style={{
        width: size,
        height: size,
        borderRadius: theme.radius.full,
        alignItems: 'center',
        justifyContent: 'center',
        backgroundColor: selected ? theme.color.accent.default : 'transparent',
        borderWidth: selected ? 0 : theme.borderWidth.thick,
        borderColor: theme.color.text.tertiary,
      }}
    >
      {selected && (
        <SymbolView
          name={{ ios: 'checkmark', android: 'check' }}
          size={theme.size.icon.sm}
          tintColor={theme.color.onAccent}
          style={{ width: theme.size.icon.sm, height: theme.size.icon.sm }}
        />
      )}
    </View>
  );
}
