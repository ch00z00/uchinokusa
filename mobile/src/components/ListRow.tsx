import type { ReactNode } from 'react';
import { Pressable, StyleSheet, View } from 'react-native';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

export type ListRowProps = {
  title: string;
  /** 右側の補足（現在の値など） */
  detail?: string;
  /** 題名の下の補足 */
  subtitle?: string;
  /** 補足の下に置くもの（注意の行など） */
  footer?: ReactNode;
  /** 項目の左に置くもの（作業の種類のアイコンなど） */
  leading?: ReactNode;
  /** 題名の右に置くもの（バッジなど） */
  accessory?: ReactNode;
  /** 項目の右端に置く操作（「取り消す」のボタンなど）。置くときは、項目そのものは押せなくする（onPress を渡さない） */
  trailing?: ReactNode;
  /** 選択中の印（一覧から1つを選ぶとき） */
  selected?: boolean;
  /** 取り消せない操作（削除・退出など）。文字を color/danger にする */
  destructive?: boolean;
  /** 一覧の中の操作（「＋ 新しい置き場所」など）。文字を color/accent にする */
  accent?: boolean;
  /** 押すと別の画面に進むなら true（右に「›」を出す） */
  navigates?: boolean;
  onPress?: () => void;
  disabled?: boolean;
  accessibilityHint?: string;
  /** 行ごと目立たせる（期限切れのタスクなど）。地を accent/subtle にし、左に status/overdue の帯を付ける。色だけに頼らず、言葉（バッジなど）もいっしょに出す */
  emphasized?: boolean;
};

/**
 * 一覧の項目（アプリの層。foundations.md の 5章）。設定の項目、世帯・メンバーの一覧、選択肢の一覧で使う。
 * 高さは 44pt 以上、背景は color/bg/surface、下に区切り線
 */
export function ListRow({
  title,
  detail,
  subtitle,
  footer,
  leading,
  accessory,
  trailing,
  selected,
  destructive,
  accent,
  navigates,
  onPress,
  disabled,
  accessibilityHint,
  emphasized,
}: ListRowProps) {
  const theme = useTheme();
  return (
    <Pressable
      // 押せない項目（右端のボタンで操作する項目など）は、読み上げでボタンと伝えない
      accessibilityRole={onPress ? 'button' : undefined}
      accessibilityState={onPress ? { disabled, selected } : { selected }}
      accessibilityHint={accessibilityHint}
      disabled={disabled || !onPress}
      onPress={onPress}
      style={({ pressed }) => [
        styles.row,
        {
          minHeight: theme.size.tapMin,
          paddingHorizontal: theme.space.lg,
          // 左の帯のぶん内側を狭め、アイコンと文字の位置をほかの行とそろえる
          paddingLeft: theme.space.lg - (emphasized ? theme.borderWidth.emphasis : 0),
          paddingVertical: theme.space.md,
          gap: theme.space.sm,
          backgroundColor: pressed
            ? theme.color.bg.subtle
            : emphasized
              ? theme.color.accent.subtle
              : theme.color.bg.surface,
          borderLeftWidth: emphasized ? theme.borderWidth.emphasis : 0,
          borderLeftColor: theme.color.status.overdue,
          borderBottomColor: theme.color.border.subtle,
          borderBottomWidth: theme.borderWidth.hairline,
        },
        disabled && styles.dimmed,
      ]}
    >
      {leading}
      <View style={[styles.main, { gap: theme.space.xs }]}>
        <View style={[styles.titleLine, { gap: theme.space.sm }]}>
          <AppText color={destructive ? 'danger' : accent ? 'accent' : 'primary'} style={styles.shrink}>
            {title}
          </AppText>
          {accessory}
        </View>
        {subtitle && (
          <AppText variant="footnote" color="secondary">
            {subtitle}
          </AppText>
        )}
        {footer}
      </View>
      {detail !== undefined && (
        <AppText color="secondary" numberOfLines={1} style={styles.shrink}>
          {detail}
        </AppText>
      )}
      {/* TODO：OS のシンボル（checkmark / chevron.right）にする（SettingsHeaderButton と同じく、アイコンの導入のあと） */}
      {selected && <AppText color="accent">✓</AppText>}
      {navigates && <AppText color="tertiary">›</AppText>}
      {trailing}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
  main: { flex: 1 },
  titleLine: { flexDirection: 'row', alignItems: 'center', flexWrap: 'wrap' },
  shrink: { flexShrink: 1 },
  dimmed: { opacity: 0.6 },
});
