import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ScrollView } from 'react-native';

import { useToast } from '@/components/Toast';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { PlantForm } from '@/features/plant/PlantForm';
import { emptyPlantForm, toPlantRequest } from '@/features/plant/plantValidation';
import { useCreatePlant } from '@/features/plant/usePlants';
import { useTheme } from '@/theme';

// PLANT-02 植物の登録（要件5-1、5-2）。植物の一覧（PLANT-01）のヘッダーの「追加」から開くモーダル（編集者以上）
export default function NewPlantScreen() {
  const { state } = useSelectedHousehold();
  if (state.status !== 'selected') return null;
  return <NewPlant householdId={state.household.id} />;
}

function NewPlant({ householdId }: { householdId: string }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  const create = useCreatePlant(householdId);

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="interactive"
      contentContainerStyle={{ padding: theme.space.lg }}
    >
      <PlantForm
        householdId={householdId}
        initial={emptyPlantForm}
        submitLabel={t('plants.create.submit')}
        pending={create.isPending}
        error={create.error}
        onSubmit={(value) =>
          create.mutate(toPlantRequest(value), {
            // TODO(16.2c)：購入日があれば、植え替えの提案（PLANT-04）を出す
            onSuccess: () => {
              toast.show(t('plants.create.done'));
              router.back();
            },
          })
        }
      />
    </ScrollView>
  );
}
