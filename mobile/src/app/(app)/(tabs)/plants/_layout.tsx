import { Stack } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { SettingsHeaderButton } from '@/components/SettingsHeaderButton';

export default function PlantsLayout() {
  const { t } = useTranslation();
  return (
    <Stack>
      <Stack.Screen
        name="index"
        options={{
          title: t('plants.title'),
          headerLargeTitleEnabled: true,
          headerRight: () => <SettingsHeaderButton />,
        }}
      />
    </Stack>
  );
}
