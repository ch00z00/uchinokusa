import { Stack } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { householdSwitcherOptions } from '@/features/household/HouseholdSwitcher';

// リンクで奥の画面（アーカイブした植物など）を直接開いても、下に植物の一覧を積み、戻れるようにする
export const unstable_settings = { initialRouteName: 'index' };

export default function PlantsLayout() {
  const { t } = useTranslation();
  return (
    <Stack>
      <Stack.Screen
        name="index"
        options={{
          title: t('plants.title'),
          headerLargeTitleEnabled: true,
          // 選択中の世帯名。押すと世帯の切り替え（HH-10）。設定は、タブバーの「設定」から開く
          ...householdSwitcherOptions,
        }}
      />
      {/* PLANT-05。タブバーを出したまま、植物のタブの中で開く */}
      <Stack.Screen name="archived" options={{ title: t('plants.archived.title') }} />
    </Stack>
  );
}
