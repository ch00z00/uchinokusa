import { View } from 'react-native';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

type Props = {
  label: string;
  /** accent：目立たせる（オーナーなど）、neutral：控えめ（一時停止中など） */
  tone?: 'accent' | 'neutral';
  /** 地が accent/subtle の上（目立たせた行など）に置くとき。地を bg/surface にして、バッジの形が見えるようにする */
  onTint?: boolean;
};

/** バッジ（アプリの層。foundations.md の 5章の Badge）。`caption`、角丸 `radius/sm`、役割の色の薄い背景 */
export function Badge({ label, tone = 'neutral', onTint = false }: Props) {
  const theme = useTheme();
  const accent = tone === 'accent';
  return (
    <View
      style={{
        alignSelf: 'flex-start',
        paddingHorizontal: theme.space.sm,
        paddingVertical: theme.space.xs,
        borderRadius: theme.radius.sm,
        backgroundColor: onTint ? theme.color.bg.surface : accent ? theme.color.accent.subtle : theme.color.bg.subtle,
      }}
    >
      <AppText variant="caption" color={accent ? 'accent' : 'secondary'}>
        {label}
      </AppText>
    </View>
  );
}
