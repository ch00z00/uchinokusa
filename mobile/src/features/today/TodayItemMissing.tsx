import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView } from 'react-native';

import { AppText } from '@/components/AppText';
import { useTheme } from '@/theme';

/** 開いたタスクが今日の一覧にない（ほかのメンバーが先に完了した、など）。読み込み中ならくるくる */
export function TodayItemMissing({ loading }: { loading: boolean }) {
  const { t } = useTranslation();
  const theme = useTheme();
  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={{ padding: theme.space.lg }}>
      {loading ? (
        <ActivityIndicator color={theme.color.text.secondary} />
      ) : (
        <AppText color="secondary">{t('today.missing')}</AppText>
      )}
    </ScrollView>
  );
}
