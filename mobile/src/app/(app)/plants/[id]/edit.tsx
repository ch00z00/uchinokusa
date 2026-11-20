import { router, useLocalSearchParams } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { useToast } from '@/components/Toast';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import type { Plant } from '@/features/plant/plantList';
import { PlantForm } from '@/features/plant/PlantForm';
import { plantToForm, toPlantRequest } from '@/features/plant/plantValidation';
import { usePlant, useUpdatePlant } from '@/features/plant/usePlants';
import { useTheme } from '@/theme';

// PLANT-02 植物の編集（要件5-6）。植物の詳細（PLANT-10）のヘッダーの「編集」から開くモーダル（編集者以上）
// TODO(16.2d)：下にアーカイブと削除を置く
export default function EditPlantScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { state } = useSelectedHousehold();
  const plant = usePlant(id);

  if (state.status !== 'selected') return null;
  if (plant.data) return <EditPlant key={plant.data.id} householdId={state.household.id} plant={plant.data} />;
  return (
    <View style={{ padding: theme.space.lg, gap: theme.space.md }}>
      {plant.isError ? (
        <>
          <AppText color="secondary">{t(errorMessageKey(plant.error))}</AppText>
          <Button title={t('common.retry')} variant="secondary" onPress={() => void plant.refetch()} />
        </>
      ) : (
        <ActivityIndicator color={theme.color.text.secondary} />
      )}
    </View>
  );
}

function EditPlant({ householdId, plant }: { householdId: string; plant: Plant }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  const update = useUpdatePlant(householdId, plant.id);

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="interactive"
      contentContainerStyle={{ padding: theme.space.lg }}
    >
      <PlantForm
        householdId={householdId}
        initial={plantToForm(plant)}
        submitLabel={t('common.save')}
        pending={update.isPending}
        error={update.error}
        onSubmit={(value) =>
          update.mutate(toPlantRequest(value), {
            onSuccess: () => {
              toast.show(t('common.saved'));
              router.back();
            },
          })
        }
      />
    </ScrollView>
  );
}
