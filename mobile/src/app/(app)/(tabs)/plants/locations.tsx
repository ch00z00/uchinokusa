import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { ListRow } from '@/components/ListRow';
import { can } from '@/features/household/permissions';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import type { Household } from '@/features/household/useHouseholds';
import { locationSummaryKeys } from '@/features/location/locationValidation';
import { useLocations } from '@/features/location/useLocations';
import { useTheme } from '@/theme';

// LOC-01 置き場所の一覧（要件7）。植物の一覧（PLANT-01）のヘッダーの「置き場所」から開く
export default function LocationsScreen() {
  const { state } = useSelectedHousehold();
  if (state.status !== 'selected') return null;
  return <Locations household={state.household} />;
}

function Locations({ household }: { household: Household }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const locations = useLocations(household.id);
  // 作成・編集・削除は編集者以上（要件4-3、4-4）。閲覧者には一覧を見せるだけ
  const canEdit = can(household.role, 'edit');

  let list;
  if (locations.data) {
    list =
      locations.data.length === 0 ? (
        <AppText color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
          {t(canEdit ? 'location.list.emptyEditor' : 'location.list.empty')}
        </AppText>
      ) : (
        <View>
          {locations.data.map((location) => (
            <ListRow
              key={location.id}
              title={location.name}
              subtitle={locationSummaryKeys(location)
                .map((key) => t(key))
                .join(t('location.list.separator'))}
              detail={t('location.list.plantCount', { n: location.plantCount })}
              navigates={canEdit}
              onPress={
                canEdit ? () => router.push({ pathname: '/locations/[id]', params: { id: location.id } }) : undefined
              }
            />
          ))}
        </View>
      );
  } else if (locations.isError) {
    list = (
      <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.md }}>
        <AppText color="secondary">{t(errorMessageKey(locations.error))}</AppText>
        <Button title={t('common.retry')} variant="secondary" onPress={() => void locations.refetch()} />
      </View>
    );
  } else {
    list = <ActivityIndicator color={theme.color.text.secondary} />;
  }

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.xl }}
    >
      {canEdit && (
        <View style={{ paddingHorizontal: theme.space.lg }}>
          <Button title={t('location.list.add')} onPress={() => router.push('/locations/new')} />
        </View>
      )}
      {list}
      <AppText variant="footnote" color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
        {t('location.list.note')}
      </AppText>
    </ScrollView>
  );
}
