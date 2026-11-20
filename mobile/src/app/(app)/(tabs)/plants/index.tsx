import { router, Stack } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, FlatList, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { FilterChips } from '@/components/FilterChips';
import { headerRightTextButton } from '@/components/HeaderTextButton';
import { ListRow } from '@/components/ListRow';
import { can } from '@/features/household/permissions';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import type { Household } from '@/features/household/useHouseholds';
import { useLocations } from '@/features/location/useLocations';
import { filterPlantsByLocation, sortPlantsByNextDue, type LocationFilter } from '@/features/plant/plantList';
import { PlantListItem } from '@/features/plant/PlantListItem';
import { useArchivedPlants, usePlants } from '@/features/plant/usePlants';
import { useDebouncedValue } from '@/lib/useDebouncedValue';
import { useTheme } from '@/theme';

// 入力が止まってから探す（入力のたびに API を呼ばない）
const searchDelayMs = 300;

// PLANT-01 植物の一覧（要件5-4、5-5）。植物のダッシュボードとして、次の予定日が近い順に並べる
export default function PlantsScreen() {
  const { state } = useSelectedHousehold();
  if (state.status !== 'selected') return null;
  // 世帯を切り替えたら、検索と絞り込みを初期化する
  return <Plants key={state.household.id} household={state.household} />;
}

function Plants({ household }: { household: Household }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const [query, setQuery] = useState('');
  const q = useDebouncedValue(query.trim(), searchDelayMs);
  const [filterKey, setFilterKey] = useState('all');
  const plants = usePlants(household.id, q);
  const locations = useLocations(household.id);
  const archived = useArchivedPlants(household.id);
  const canEdit = can(household.role, 'edit');

  const locationNames = new Map(locations.data?.map((l) => [l.id, l.name]));
  // 選んでいた置き場所が削除されたら、「すべて」に戻す
  const filter: LocationFilter =
    filterKey === 'unset'
      ? { kind: 'unset' }
      : locationNames.has(filterKey)
        ? { kind: 'location', id: filterKey }
        : { kind: 'all' };
  const selectedKey = filter.kind === 'location' ? filter.id : filter.kind;
  const visible = plants.data ? sortPlantsByNextDue(filterPlantsByLocation(plants.data, filter)) : [];
  const narrowed = q !== '' || filter.kind !== 'all';

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
  } else if (narrowed) {
    empty = <AppText color="secondary">{t('plants.list.noMatch')}</AppText>;
  } else {
    empty = canEdit ? (
      <View style={{ gap: theme.space.md }}>
        <AppText color="secondary">{t('plants.list.emptyEditor')}</AppText>
        <Button title={t('plants.create.title')} onPress={() => router.push('/plants/new')} />
      </View>
    ) : (
      <AppText color="secondary">{t('plants.list.empty')}</AppText>
    );
  }

  return (
    <>
      <Stack.Screen
        options={{
          // OS のヘッダーの検索バー（名前での検索。ひらがな・カタカナなどを区別しない探し方は API に任せる）
          headerSearchBarOptions: {
            placeholder: t('plants.list.searchPlaceholder'),
            onChangeText: (e) => setQuery(e.nativeEvent.text),
            onCancelButtonPress: () => setQuery(''),
          },
          // 登録（PLANT-02）は編集者以上
          ...(canEdit
            ? headerRightTextButton({ title: t('plants.create.open'), onPress: () => router.push('/plants/new') })
            : {}),
        }}
      />
      <FlatList
        contentInsetAdjustmentBehavior="automatic"
        keyboardDismissMode="on-drag"
        data={visible}
        keyExtractor={(plant) => plant.id}
        renderItem={({ item }) => (
          <PlantListItem plant={item} locationName={item.locationId ? locationNames.get(item.locationId) : undefined} />
        )}
        // 置き場所が1つもなければ、絞り込みを出さない
        ListHeaderComponent={
          locations.data && locations.data.length > 0 ? (
            <FilterChips
              accessibilityLabel={t('plants.list.filterLabel')}
              chips={[
                { key: 'all', label: t('plants.list.filterAll') },
                ...locations.data.map((l) => ({ key: l.id, label: l.name })),
                { key: 'unset', label: t('plants.list.locationUnset') },
              ]}
              selectedKey={selectedKey}
              onSelect={setFilterKey}
            />
          ) : undefined
        }
        // アーカイブした植物（PLANT-05）の入口。たまにしか使わないので、ヘッダーではなく一覧のいちばん下に置き、
        // アーカイブした植物があるときだけ出す。検索・絞り込みの結果には混ぜない（2026-11-21 に決定）
        ListFooterComponent={
          !narrowed && plants.data && archived.data && archived.data.length > 0 ? (
            <View style={{ marginTop: theme.space['2xl'] }}>
              <ListRow
                title={t('plants.list.archivedEntry')}
                detail={String(archived.data.length)}
                navigates
                onPress={() => router.push('/plants/archived')}
              />
            </View>
          ) : undefined
        }
        ListEmptyComponent={<View style={{ padding: theme.space.lg }}>{empty}</View>}
      />
    </>
  );
}
