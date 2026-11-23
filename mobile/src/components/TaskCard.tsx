import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Pressable, StyleSheet, View } from 'react-native';

import type { CareKind } from '@/features/task/careTask';
import { useTheme } from '@/theme';

import { AppText } from './AppText';
import { CareKindIcon } from './CareKindIcon';
import { OverdueBadge } from './OverdueBadge';

/** カードに出す提案（「今日は不要」・「条件つき」）。種類の名前と文 */
export type TaskCardSuggestion = { kind: 'avoid' | 'conditional'; text: string };

type Props = {
  kind: CareKind;
  /** 植物名 */
  title: string;
  /** 作業の名前・置き場所・予定日をつないだ1行 */
  meta: string;
  /** 予定日を過ぎた日数。0 なら出さない */
  overdueDays: number;
  /** 実行条件（例：土の表面が乾いていたら） */
  conditionText?: string;
  suggestion?: TaskCardSuggestion;
  /** 右に置く操作（完了のボタン。17.2 で足す。閲覧者には渡さない） */
  trailing?: ReactNode;
  onPress?: () => void;
  /** 長押し（スキップなどのメニュー。Android。iOS はリンクのメニューで出す） */
  onLongPress?: () => void;
  accessibilityHint?: string;
};

/**
 * 今日やることのタスク（アプリの層。foundations.md の 5章の TaskCard。TODAY-01、要件9-2〜9-4・19-4）。
 * 「今日は不要」は地を bg/subtle にして通常のタスクと区別し、種類の名前でも示す（色だけに頼らない）
 */
export function TaskCard({
  kind,
  title,
  meta,
  overdueDays,
  conditionText,
  suggestion,
  trailing,
  onPress,
  onLongPress,
  accessibilityHint,
}: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  const notNeeded = suggestion?.kind === 'avoid';
  return (
    <Pressable
      accessibilityRole={onPress ? 'button' : undefined}
      accessibilityHint={accessibilityHint}
      disabled={!onPress}
      onPress={onPress}
      onLongPress={onLongPress}
      style={({ pressed }) => [
        styles.card,
        {
          gap: theme.space.md,
          padding: theme.space.lg,
          borderRadius: theme.radius.lg,
          backgroundColor: notNeeded || pressed ? theme.color.bg.subtle : theme.color.bg.surface,
          borderWidth: theme.borderWidth.hairline,
          borderColor: theme.color.border.subtle,
        },
      ]}
    >
      <View style={{ paddingTop: theme.space.xs }}>
        <CareKindIcon kind={kind} size="md" />
      </View>
      <View style={[styles.main, { gap: theme.space.xs }]}>
        {suggestion && (
          <View style={{ gap: theme.space.xs, paddingBottom: theme.space.xs }}>
            <AppText variant="subheadline" style={{ color: theme.color.suggestion[suggestion.kind] }}>
              {t(`suggestion.kind.${suggestion.kind}`)}
            </AppText>
            <AppText variant="callout" color="secondary">
              {suggestion.text}
            </AppText>
          </View>
        )}
        <View style={[styles.titleLine, { gap: theme.space.sm }]}>
          <AppText variant="headline" style={styles.shrink}>
            {title}
          </AppText>
          {overdueDays > 0 && <OverdueBadge days={overdueDays} />}
        </View>
        <AppText variant="subheadline" color="secondary">
          {meta}
        </AppText>
        {conditionText && <AppText variant="callout">{conditionText}</AppText>}
      </View>
      {trailing && <View style={styles.trailing}>{trailing}</View>}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: { flexDirection: 'row', alignItems: 'flex-start' },
  main: { flex: 1 },
  titleLine: { flexDirection: 'row', alignItems: 'center', flexWrap: 'wrap' },
  shrink: { flexShrink: 1 },
  trailing: { alignSelf: 'center' },
});
