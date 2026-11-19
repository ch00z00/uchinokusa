import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';

import { ListRow } from '@/components/ListRow';
import { RoleBadge } from '@/components/RoleBadge';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { useHouseholds } from '@/features/household/useHouseholds';
import { useTheme } from '@/theme';

// HH-10 世帯の切り替え（OS のシート）。ヘッダーの世帯名から開く。世帯が1つだけのときも開ける（2つ目の世帯の入口）
export default function SwitchHouseholdScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const households = useHouseholds();
  const { state, select } = useSelectedHousehold();
  const selectedId = state.status === 'selected' ? state.household.id : null;

  return (
    // シートの見出しの下に一覧がもぐらないように、見出しの分だけ中身を下げる
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.xl }}
    >
      <View>
        {households.data?.map((household) => (
          <ListRow
            key={household.id}
            title={household.name}
            accessory={<RoleBadge role={household.role} />}
            selected={household.id === selectedId}
            onPress={() => {
              select(household.id);
              router.back();
            }}
          />
        ))}
      </View>
      <View>
        <ListRow title={t('household.create')} navigates onPress={() => router.replace('/households/new')} />
        <ListRow title={t('household.join')} navigates onPress={() => router.replace('/join')} />
      </View>
    </ScrollView>
  );
}
