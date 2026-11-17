import { Stack } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { SettingsHeaderButton } from '@/components/SettingsHeaderButton';
import { HouseholdSwitcher } from '@/features/household/HouseholdSwitcher';

export default function TodayLayout() {
  const { t } = useTranslation();
  return (
    <Stack>
      <Stack.Screen
        name="index"
        options={{
          title: t('today.title'),
          headerLargeTitleEnabled: true,
          // 選択中の世帯名。押すと世帯の切り替え（HH-10）
          headerLeft: () => <HouseholdSwitcher />,
          headerRight: () => <SettingsHeaderButton />,
        }}
      />
    </Stack>
  );
}
