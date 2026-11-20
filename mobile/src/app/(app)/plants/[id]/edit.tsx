import { router, useLocalSearchParams } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { ListRow } from '@/components/ListRow';
import { useToast } from '@/components/Toast';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import type { Plant } from '@/features/plant/plantList';
import { PlantForm } from '@/features/plant/PlantForm';
import { plantToForm, toPlantRequest } from '@/features/plant/plantValidation';
import { useArchivePlant, useDeletePlant, usePlant, useUpdatePlant } from '@/features/plant/usePlants';
import { confirm } from '@/lib/confirm';
import { useTheme } from '@/theme';

// PLANT-02 植物の編集（要件5-6）。植物の詳細（PLANT-10）のヘッダーの「編集」から開くモーダル（編集者以上）
// 下に、アーカイブ（要件5-8）と削除（要件5-7）を置く
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
  const archive = useArchivePlant(householdId, plant.id);
  const remove = useDeletePlant(householdId, plant.id);
  const archived = plant.archivedAt !== undefined;
  const busy = archive.isPending || remove.isPending;

  // アーカイブ・削除のあとは、植物の詳細ごと閉じて、元の一覧へ戻る
  const backToList = () => router.dismissTo(archived ? '/plants/archived' : '/plants');

  const archivePlant = async () => {
    const ok = await confirm({
      title: t('plants.archive.confirmTitle', { name: plant.name }),
      message: t('plants.archive.confirmMessage'),
      confirmLabel: t('plants.archive.confirm'),
      cancelLabel: t('common.cancel'),
    });
    if (!ok) return;
    archive.mutate(undefined, {
      onSuccess: () => {
        toast.show(t('plants.archive.done'));
        backToList();
      },
    });
  };

  const deletePlant = async () => {
    const ok = await confirm({
      title: t('plants.delete.confirmTitle', { name: plant.name }),
      message: t('plants.delete.confirmMessage'),
      confirmLabel: t('plants.delete.confirm'),
      cancelLabel: t('common.cancel'),
      destructive: true,
    });
    if (!ok) return;
    remove.mutate(undefined, {
      onSuccess: () => {
        toast.show(t('plants.delete.done'));
        backToList();
      },
    });
  };

  const actionError = archive.error ?? remove.error;

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="interactive"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space['2xl'] }}
    >
      <View style={{ paddingHorizontal: theme.space.lg }}>
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
      </View>

      <View>
        {/* アーカイブ中の植物は、詳細の「アーカイブを解除」で戻す */}
        {!archived && (
          <ListRow
            title={t('plants.archive.action')}
            subtitle={t('plants.archive.hint')}
            disabled={busy}
            onPress={() => void archivePlant()}
          />
        )}
        <ListRow title={t('plants.delete.action')} destructive disabled={busy} onPress={() => void deletePlant()} />
        {actionError && (
          <AppText variant="footnote" color="danger" style={{ padding: theme.space.lg }}>
            {t(errorMessageKey(actionError))}
          </AppText>
        )}
      </View>
    </ScrollView>
  );
}
