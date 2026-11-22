import { SymbolView, type SymbolViewProps } from 'expo-symbols';

import { normalizeCareKind, type CareKind } from '@/features/task/careTask';
import { useTheme } from '@/theme';

/**
 * 仮置きのシンボル（foundations.md の 4.1）。オリジナルのアイコンができたら SVG に替える。
 * iOS は SF Symbols、Android は Material Symbols
 */
const symbols: Record<CareKind, SymbolViewProps['name']> = {
  watering: { ios: 'drop', android: 'water_drop' },
  fertilizing: { ios: 'sparkles', android: 'compost' },
  repotting: { ios: 'arrow.up.bin', android: 'potted_plant' },
  pruning: { ios: 'scissors', android: 'content_cut' },
  other: { ios: 'leaf', android: 'eco' },
};

type Props = {
  kind: CareKind | string;
  /** sm 16pt（バッジの中）・md 20pt（一覧・TaskCard）・lg 28pt（植物の詳細のタスク） */
  size?: 'sm' | 'md' | 'lg';
};

/**
 * 作業の種類のアイコン（アプリの層。foundations.md の 5章の CareKindIcon）。作業の種類の色（color/care/*）で塗る。
 * 色だけに頼らないよう、使う場所では種類の名前もいっしょに出す。読み上げでは名前のほうを読ませる
 */
export function CareKindIcon({ kind, size = 'md' }: Props) {
  const theme = useTheme();
  const normalized = normalizeCareKind(kind);
  const px = theme.size.icon[size];
  return (
    <SymbolView
      name={symbols[normalized]}
      size={px}
      tintColor={theme.color.care[normalized]}
      style={{ width: px, height: px }}
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
    />
  );
}
