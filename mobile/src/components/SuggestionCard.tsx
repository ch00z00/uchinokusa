import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { components } from '@/api/schema';
import { useTheme } from '@/theme';

import { AppText } from './AppText';

type SuggestionKind = components['schemas']['SuggestionKind'];

type Props = {
  kind: SuggestionKind;
  /** 本文（「ポトスの植え替えは、2027年4月1日(木)がおすすめです」） */
  message: string;
  /** 理由（要件17-7） */
  reason: string;
  /** 採用・見送りなどのボタン。閲覧者には渡さない */
  children?: ReactNode;
};

/**
 * 提案のカード（アプリの層。foundations.md の 5章の SuggestionCard）。
 * 提案の種類は、色だけに頼らず、種類の名前（「おすすめ」など）でも示す。
 * TODO：提案の種類のオリジナルのアイコン（foundations.md の 4.1）ができたら、名前の左に置く
 */
export function SuggestionCard({ kind, message, reason, children }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  // 知らない種類は、おすすめとして扱う（API の SuggestionKind の説明。値は今後も追加されうる）
  const known: SuggestionKind = kind === 'avoid' || kind === 'conditional' ? kind : 'do';
  const color = theme.color.suggestion[known];
  return (
    <View
      style={{
        gap: theme.space.md,
        padding: theme.space.lg,
        borderRadius: theme.radius.lg,
        backgroundColor: theme.color.bg.surface,
        borderWidth: theme.borderWidth.hairline,
        borderColor: theme.color.border.subtle,
      }}
    >
      <AppText variant="subheadline" style={{ color }}>
        {t(`suggestion.kind.${known}`)}
      </AppText>
      <AppText variant="body">{message}</AppText>
      <AppText variant="callout" color="secondary">
        {reason}
      </AppText>
      {children && <View style={{ gap: theme.space.sm }}>{children}</View>}
    </View>
  );
}
