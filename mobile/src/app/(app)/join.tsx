import { useTranslation } from 'react-i18next';
import { ScrollView } from 'react-native';

import { AppText } from '@/components/AppText';
import { useTheme } from '@/theme';

// 招待から参加する。招待コードの入力（INV-10）と QR コードの読み取り（INV-11）は、タスク 15.2 で作る
export default function JoinScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={{ padding: theme.space.lg }}>
      <AppText color="secondary">{t('join.linkGuide')}</AppText>
    </ScrollView>
  );
}
