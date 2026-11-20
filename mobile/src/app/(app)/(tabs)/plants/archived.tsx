import { useTranslation } from 'react-i18next';
import { ActivityIndicator, FlatList, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { useLocations } from '@/features/location/useLocations';
import { PlantListItem } from '@/features/plant/PlantListItem';
import { useArchivedPlants } from '@/features/plant/usePlants';
import { useTheme } from '@/theme';

// PLANT-05 アーカイブした植物（要件5-8）。植物の一覧のいちばん下から開く。
// 開くと詳細（PLANT-10）で記録とメモを見られ、編集者以上はそこでアーカイブを解除できる
export default function ArchivedPlantsScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { state } = useSelectedHousehold();
  const householdId = state.status === 'selected' ? state.household.id : '';
  const plants = useArchivedPlants(householdId);
  const locations = useLocations(householdId);
  const locationNames = new Map(locations.data?.map((l) => [l.id, l.name]));

  let empty;
  if (plants.isError) {
    empty = (
      <View style={{ gap: theme.space.md }}>
        <AppText color="secondary">{t(errorMessageKey(plants.error))}</AppText>
        <Button title={t('common.retry')} variant="secondary" onPress={() => void plants.refetch()} />
      </View>
    );
  } else if (!plants.data) {
    empty = <ActivityIndicator color={theme.color.text.secondary} />;
  } else {
    empty = <AppText color="secondary">{t('plants.archived.empty')}</AppText>;
  }

  return (
    <FlatList
      contentInsetAdjustmentBehavior="automatic"
      data={plants.data ?? []}
      keyExtractor={(plant) => plant.id}
      renderItem={({ item }) => (
        <PlantListItem
          plant={item}
          locationName={item.locationId ? locationNames.get(item.locationId) : undefined}
          archived
        />
      )}
      ListHeaderComponent={
        plants.data && plants.data.length > 0 ? (
          <AppText variant="footnote" color="secondary" style={{ padding: theme.space.lg }}>
            {t('plants.archived.lead')}
          </AppText>
        ) : undefined
      }
      ListEmptyComponent={<View style={{ padding: theme.space.lg }}>{empty}</View>}
    />
  );
}
