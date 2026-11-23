import type { ReactNode } from 'react';
import { View } from 'react-native';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

type Props = {
  title: string;
  message?: string;
  /** 次の行動のボタン（「植物を登録」など）。閲覧者には渡さない */
  children?: ReactNode;
};

/**
 * 空の状態（アプリの層。foundations.md の 5章の EmptyState）。見出し（`title3`）と説明（`subheadline`）と任意のボタン。
 * TODO：空の状態のオリジナルのイラスト（foundations.md の 4.1）ができたら、見出しの上に置く
 */
export function EmptyState({ title, message, children }: Props) {
  const theme = useTheme();
  return (
    <View style={{ alignItems: 'center', gap: theme.space.sm, padding: theme.space.xl }}>
      <AppText variant="title3" style={{ textAlign: 'center' }}>
        {title}
      </AppText>
      {message && (
        <AppText variant="subheadline" color="secondary" style={{ textAlign: 'center' }}>
          {message}
        </AppText>
      )}
      {children && <View style={{ alignSelf: 'stretch', paddingTop: theme.space.md }}>{children}</View>}
    </View>
  );
}
