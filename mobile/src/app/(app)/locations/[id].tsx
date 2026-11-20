import { router, useLocalSearchParams } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { ListRow } from '@/components/ListRow';
import { useToast } from '@/components/Toast';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { locationToForm, type Location } from '@/features/location/locationValidation';
import { LocationForm } from '@/features/location/LocationForm';
import { useDeleteLocation, useLocations, useUpdateLocation } from '@/features/location/useLocations';
import { confirm } from '@/lib/confirm';
import { useTheme } from '@/theme';

// LOC-02 置き場所の編集と削除（要件7-2〜7-4）。置き場所の一覧（LOC-01）の項目から開くモーダル（編集者以上）
export default function EditLocationScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { state } = useSelectedHousehold();
  const householdId = state.status === 'selected' ? state.household.id : '';
  // 一覧と同じ取得を使う（一覧から開くので、ふつうは取得済み）
  const locations = useLocations(householdId);

  if (state.status !== 'selected') return null;
  const location = locations.data?.find((l) => l.id === id);
  if (location) return <EditLocation key={location.id} householdId={householdId} location={location} />;

  return (
    <View style={{ padding: theme.space.lg, gap: theme.space.md }}>
      {locations.data ? (
        // ほかのメンバーが削除したなど
        <AppText color="secondary">{t('location.edit.notFound')}</AppText>
      ) : locations.isError ? (
        <>
          <AppText color="secondary">{t(errorMessageKey(locations.error))}</AppText>
          <Button title={t('common.retry')} variant="secondary" onPress={() => void locations.refetch()} />
        </>
      ) : (
        <ActivityIndicator color={theme.color.text.secondary} />
      )}
    </View>
  );
}

function EditLocation({ householdId, location }: { householdId: string; location: Location }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  const update = useUpdateLocation(householdId, location.id);
  const remove = useDeleteLocation(householdId, location.id);

  const confirmDelete = async () => {
    const ok = await confirm({
      title: t('location.delete.title', { name: location.name }),
      // 紐づく植物は消さず、「置き場所未設定」にする（要件7-4）。数を示してから確かめる
      message:
        location.plantCount > 0
          ? t('location.delete.messageWithPlants', { n: location.plantCount })
          : t('location.delete.message'),
      confirmLabel: t('location.delete.confirm'),
      cancelLabel: t('common.cancel'),
      destructive: true,
    });
    if (!ok) return;
    remove.mutate(undefined, {
      onSuccess: () => {
        toast.show(t('location.delete.done'));
        router.back();
      },
    });
  };

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="interactive"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.xl }}
    >
      <View style={{ paddingHorizontal: theme.space.lg }}>
        <LocationForm
          initial={locationToForm(location)}
          submitLabel={t('common.save')}
          pending={update.isPending}
          error={update.error}
          // 環境の項目は、空文字を送ると未設定に戻る
          onSubmit={(value) =>
            update.mutate(value, {
              onSuccess: () => {
                toast.show(t('common.saved'));
                router.back();
              },
            })
          }
        />
      </View>
      <View>
        <ListRow
          title={t('location.delete.action')}
          destructive
          disabled={remove.isPending}
          onPress={() => void confirmDelete()}
        />
        {remove.isError && (
          <AppText variant="footnote" color="danger" style={{ padding: theme.space.lg }}>
            {t(errorMessageKey(remove.error))}
          </AppText>
        )}
      </View>
    </ScrollView>
  );
}
