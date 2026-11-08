import { Stack } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { SettingsHeaderButton } from '@/components/SettingsHeaderButton';

export default function TodayLayout() {
  const { t } = useTranslation();
  return (
    <Stack>
      <Stack.Screen
        name="index"
        options={{
          title: t('today.title'),
          headerLargeTitleEnabled: true,
          headerRight: () => <SettingsHeaderButton />,
        }}
      />
    </Stack>
  );
}
