import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ScrollView } from 'react-native';

import { HouseholdForm } from '@/features/household/HouseholdForm';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { useCreateHousehold } from '@/features/household/useHouseholds';
import { useTheme } from '@/theme';

// ONB-02 世帯の作成。はじめての選択（ONB-01）と、世帯の切り替え（HH-10）から開く
export default function NewHouseholdScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const create = useCreateHousehold();
  const { select } = useSelectedHousehold();

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="interactive"
      contentContainerStyle={{ padding: theme.space.lg }}
    >
      <HouseholdForm
        initial={{ name: '', regionPrefecture: '', regionCity: '' }}
        submitLabel={t('household.form.create')}
        pending={create.isPending}
        error={create.error}
        onSubmit={({ name, regionPrefecture, regionCity }) =>
          create.mutate(
            // 地域の空は、送らずに未設定にする
            { name, regionPrefecture: regionPrefecture || undefined, regionCity: regionCity || undefined },
            {
              onSuccess: (household) => {
                // 作った世帯を表示し、「今日」へ（植物がまだないので、空の状態になる）
                select(household.id);
                router.dismissTo('/today');
              },
            },
          )
        }
      />
    </ScrollView>
  );
}
