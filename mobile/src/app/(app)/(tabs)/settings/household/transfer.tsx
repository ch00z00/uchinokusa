import { router, useLocalSearchParams } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { ListRow } from '@/components/ListRow';
import { RoleBadge } from '@/components/RoleBadge';
import { useMe } from '@/features/account/useMe';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { useHouseholds, useMembers, useTransferOwnership, type Household } from '@/features/household/useHouseholds';
import { confirm } from '@/lib/confirm';
import { useTheme } from '@/theme';

// HH-03 オーナー権限の移譲。退出（要件2-8）の途中からは選択中の世帯で、
// アカウントの削除（要件1-7、SET-04）からは householdId で対象の世帯を指定して開く（選択中の世帯は変えない）
export default function TransferScreen() {
  const { householdId } = useLocalSearchParams<{ householdId?: string }>();
  const { state } = useSelectedHousehold();
  const households = useHouseholds();
  const household = householdId
    ? households.data?.find((h) => h.id === householdId)
    : state.status === 'selected'
      ? state.household
      : undefined;
  if (!household) return null;
  return <Transfer household={household} />;
}

function Transfer({ household }: { household: Household }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const me = useMe();
  const members = useMembers(household.id);
  const transfer = useTransferOwnership(household.id);
  const candidates = members.data?.filter((m) => m.userId !== me.data?.id) ?? [];

  const choose = async (userId: string, name: string) => {
    const ok = await confirm({
      title: t('household.transfer.confirmTitle', { name }),
      message: t('household.transfer.confirmMessage'),
      confirmLabel: t('household.transfer.action'),
      cancelLabel: t('common.cancel'),
    });
    if (ok) transfer.mutate(userId, { onSuccess: () => router.back() });
  };

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.lg }}
    >
      <AppText color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
        {t('household.transfer.lead')}
      </AppText>
      {members.isPending && <ActivityIndicator color={theme.color.text.secondary} />}
      <View>
        {candidates.map((m) => (
          <ListRow
            key={m.userId}
            title={m.displayName}
            accessory={<RoleBadge role={m.role} />}
            disabled={transfer.isPending}
            onPress={() => void choose(m.userId, m.displayName)}
          />
        ))}
      </View>
      {(members.error ?? transfer.error) && (
        <AppText variant="footnote" color="danger" style={{ paddingHorizontal: theme.space.lg }}>
          {t(errorMessageKey(members.error ?? transfer.error))}
        </AppText>
      )}
    </ScrollView>
  );
}
