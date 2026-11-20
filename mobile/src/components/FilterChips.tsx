import { Pressable, ScrollView } from 'react-native';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

export type FilterChip = { key: string; label: string };

type Props = {
  chips: readonly FilterChip[];
  selectedKey: string;
  onSelect: (key: string) => void;
  /** 並び全体の読み上げの名前（「置き場所で絞り込む」など） */
  accessibilityLabel: string;
};

/**
 * 絞り込みのチップ（アプリの層。foundations.md の 5章の FilterChips）。横に流れる並びから1つを選ぶ。
 * チップの高さは 36pt で、上下の余白を足して押せる領域を 44pt 以上にする
 */
export function FilterChips({ chips, selectedKey, onSelect, accessibilityLabel }: Props) {
  const theme = useTheme();
  const chipHeight = 36;
  const slop = (theme.size.tapMin - chipHeight) / 2;
  return (
    <ScrollView
      horizontal
      showsHorizontalScrollIndicator={false}
      accessibilityLabel={accessibilityLabel}
      contentContainerStyle={{ paddingHorizontal: theme.space.lg, paddingVertical: slop, gap: theme.space.sm }}
    >
      {chips.map((chip) => {
        const selected = chip.key === selectedKey;
        return (
          <Pressable
            key={chip.key}
            accessibilityRole="button"
            accessibilityState={{ selected }}
            hitSlop={{ top: slop, bottom: slop }}
            onPress={() => onSelect(chip.key)}
            style={({ pressed }) => ({
              height: chipHeight,
              justifyContent: 'center',
              paddingHorizontal: theme.space.md,
              borderRadius: theme.radius.full,
              backgroundColor: selected ? theme.color.accent.default : theme.color.bg.subtle,
              opacity: pressed ? 0.7 : 1,
            })}
          >
            <AppText
              variant="subheadline"
              style={{ color: selected ? theme.color.onAccent : theme.color.text.primary }}
            >
              {chip.label}
            </AppText>
          </Pressable>
        );
      })}
    </ScrollView>
  );
}
