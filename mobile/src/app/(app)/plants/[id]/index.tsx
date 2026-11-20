import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { headerRightTextButton } from '@/components/HeaderTextButton';
import { useToast } from '@/components/Toast';
import { can } from '@/features/household/permissions';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import type { Plant } from '@/features/plant/plantList';
import { usePlant, useUnarchivePlant } from '@/features/plant/usePlants';
import { formatDate } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// PLANT-10 植物の詳細。いまは名前・編集の入口・アーカイブ中の表示だけ。中身はタスク 17・18 で作る
export default function PlantDetailScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { state } = useSelectedHousehold();
  const plant = usePlant(id);
  const householdId = state.status === 'selected' ? state.household.id : '';
  const canEdit = state.status === 'selected' && can(state.household.role, 'edit');

  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={{ padding: theme.space.lg }}>
      <Stack.Screen
        options={{
          title: plant.data?.name ?? '',
          // 編集（PLANT-02）は編集者以上。アーカイブした植物も、削除のために開ける
          ...(canEdit && plant.data
            ? headerRightTextButton({
                title: t('plants.edit.open'),
                onPress: () => router.push({ pathname: '/plants/[id]/edit', params: { id } }),
              })
            : {}),
        }}
      />
      {plant.isError ? (
        <AppText color="secondary">{t(errorMessageKey(plant.error))}</AppText>
      ) : !plant.data ? (
        <ActivityIndicator color={theme.color.text.secondary} />
      ) : plant.data.archivedAt ? (
        <ArchivedNotice householdId={householdId} plant={plant.data} canEdit={canEdit} />
      ) : null}
    </ScrollView>
  );
}

/** アーカイブ中の植物の表示（PLANT-10）。編集者以上には、アーカイブの解除を出す */
function ArchivedNotice({ householdId, plant, canEdit }: { householdId: string; plant: Plant; canEdit: boolean }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  const unarchive = useUnarchivePlant(householdId, plant.id);

  return (
    <View
      style={{
        gap: theme.space.md,
        padding: theme.space.lg,
        borderRadius: theme.radius.md,
        backgroundColor: theme.color.bg.subtle,
      }}
    >
      <AppText variant="headline">{t('plants.archive.badge')}</AppText>
      <AppText color="secondary">
        {t('plants.archive.notice', { date: plant.archivedAt ? formatDate(plant.archivedAt) : '' })}
      </AppText>
      {canEdit && (
        <Button
          title={t('plants.archive.unarchive')}
          variant="secondary"
          loading={unarchive.isPending}
          onPress={() => unarchive.mutate(undefined, { onSuccess: () => toast.show(t('plants.archive.unarchived')) })}
        />
      )}
      {unarchive.error && (
        <AppText variant="footnote" color="danger">
          {t(errorMessageKey(unarchive.error))}
        </AppText>
      )}
    </View>
  );
}
