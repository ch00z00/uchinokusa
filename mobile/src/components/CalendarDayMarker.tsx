import { View } from 'react-native';

import type { MarkerStatus } from '@/features/calendar/calendarDays';
import type { CareKind } from '@/features/task/careTask';
import { useTheme } from '@/theme';

import { CareKindIcon } from './CareKindIcon';

type Props = { kind: CareKind; status: MarkerStatus };

/** 印の円の直径。日のマス（幅は約 50pt）に 2つ並べて入る大きさ */
export const markerSize = 18;

/**
 * カレンダーの日のマスの印（アプリの層。foundations.md の 5章の CalendarDayMarker。PLANT-15、要件14-2）。
 * 作業の種類はアイコンの形と色、状態は円の形で示す（色だけに頼らない）。
 * - 完了：種類の色で塗った円に、地の色のアイコン
 * - 予定：種類の色の枠の円に、種類の色のアイコン
 * - スキップ：点線の枠の円に、薄い色のアイコン（しなかったので、種類の色を使わない）
 */
export function CalendarDayMarker({ kind, status }: Props) {
  const theme = useTheme();
  const care = theme.color.care[kind];
  const circle = {
    width: markerSize,
    height: markerSize,
    borderRadius: theme.radius.full,
    alignItems: 'center' as const,
    justifyContent: 'center' as const,
  };
  if (status === 'completed') {
    return (
      <View style={[circle, { backgroundColor: care }]}>
        <CareKindIcon kind={kind} size="xs" color={theme.color.bg.surface} />
      </View>
    );
  }
  if (status === 'planned') {
    return (
      <View style={[circle, { borderWidth: theme.borderWidth.hairline, borderColor: care }]}>
        <CareKindIcon kind={kind} size="xs" />
      </View>
    );
  }
  return (
    <View
      style={[
        circle,
        { borderWidth: theme.borderWidth.hairline, borderColor: theme.color.text.tertiary, borderStyle: 'dashed' },
      ]}
    >
      <CareKindIcon kind={kind} size="xs" color={theme.color.text.tertiary} />
    </View>
  );
}
