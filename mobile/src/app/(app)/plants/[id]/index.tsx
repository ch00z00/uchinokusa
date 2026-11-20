import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { headerRightTextButton } from '@/components/HeaderTextButton';
import { can } from '@/features/household/permissions';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { usePlant } from '@/features/plant/usePlants';
import { useTheme } from '@/theme';

// PLANT-10 植物の詳細。いまは名前と編集の入口だけ。中身はタスク 17・18 で作る
export default function PlantDetailScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { state } = useSelectedHousehold();
  const plant = usePlant(id);
  const canEdit = state.status === 'selected' && can(state.household.role, 'edit');

  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={{ padding: theme.space.lg }}>
      <Stack.Screen
        options={{
          title: plant.data?.name ?? '',
          // 編集（PLANT-02）は編集者以上
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
      ) : null}
    </ScrollView>
  );
}
