import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';

import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { useTheme } from '@/theme';

// ONB-01 はじめての選択。どの世帯にも属していないときに出す（要件2-2）
export default function OnboardingScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ padding: theme.space.lg, gap: theme.space['2xl'] }}
    >
      <View style={{ gap: theme.space.sm }}>
        <AppText variant="title2">{t('onboarding.heading')}</AppText>
        <AppText variant="callout" color="secondary">
          {t('onboarding.lead')}
        </AppText>
      </View>
      <View style={{ gap: theme.space.md }}>
        <Button title={t('household.create')} onPress={() => router.push('/households/new')} />
        <Button title={t('household.join')} variant="secondary" onPress={() => router.push('/join')} />
      </View>
    </ScrollView>
  );
}
