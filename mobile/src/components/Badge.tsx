import { View } from 'react-native';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

type Props = {
  label: string;
  /** accent：目立たせる（オーナーなど）、neutral：控えめ（一時停止中など） */
  tone?: 'accent' | 'neutral';
};

/** バッジ（アプリの層。foundations.md の 5章の Badge）。`caption`、角丸 `radius/sm`、役割の色の薄い背景 */
export function Badge({ label, tone = 'neutral' }: Props) {
  const theme = useTheme();
  const accent = tone === 'accent';
  return (
    <View
      style={{
        alignSelf: 'flex-start',
        paddingHorizontal: theme.space.sm,
        paddingVertical: theme.space.xs,
        borderRadius: theme.radius.sm,
        backgroundColor: accent ? theme.color.accent.subtle : theme.color.bg.subtle,
      }}
    >
      <AppText variant="caption" color={accent ? 'accent' : 'secondary'}>
        {label}
      </AppText>
    </View>
  );
}
