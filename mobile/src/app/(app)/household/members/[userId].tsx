import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { ListRow } from '@/components/ListRow';
import { normalizeRole } from '@/features/household/permissions';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import {
  useMembers,
  useRemoveMember,
  useTransferOwnership,
  useUpdateMemberRole,
  type Household,
  type Member,
} from '@/features/household/useHouseholds';
import { confirm } from '@/lib/confirm';
import { useTheme } from '@/theme';

// HH-02 のメンバーの操作（オーナーだけが開く）：ロールの変更、オーナー権限の移譲、世帯から外す
export default function MemberScreen() {
  const { userId } = useLocalSearchParams<{ userId: string }>();
  const { state } = useSelectedHousehold();
  const household = state.status === 'selected' ? state.household : null;
  const members = useMembers(household?.id ?? '');
  const member = members.data?.find((m) => m.userId === userId);
  if (!household || !member) return null;
  return <MemberActions household={household} member={member} />;
}

function MemberActions({ household, member }: { household: Household; member: Member }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const updateRole = useUpdateMemberRole(household.id);
  const remove = useRemoveMember(household.id);
  const transfer = useTransferOwnership(household.id);
  const role = normalizeRole(member.role);
  const busy = updateRole.isPending || remove.isPending || transfer.isPending;

  const transferOwnership = async () => {
    const ok = await confirm({
      title: t('household.transfer.confirmTitle', { name: member.displayName }),
      message: t('household.transfer.confirmMessage'),
      confirmLabel: t('household.transfer.action'),
      cancelLabel: t('common.cancel'),
    });
    // 移したあとは編集者になり、メンバーの操作はできなくなるので、世帯の設定に戻る
    if (ok) transfer.mutate(member.userId, { onSuccess: () => router.dismissTo('/household') });
  };

  const removeMember = async () => {
    const ok = await confirm({
      title: t('household.removeMember.confirmTitle', { name: member.displayName }),
      // 要件4-7：外したメンバーの過去の記録は、名前つきで残る
      message: t('household.removeMember.confirmMessage'),
      confirmLabel: t('household.removeMember.action'),
      cancelLabel: t('common.cancel'),
      destructive: true,
    });
    if (ok) remove.mutate({ userId: member.userId, self: false }, { onSuccess: () => router.back() });
  };

  const error = updateRole.error ?? remove.error ?? transfer.error;

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.xl }}
    >
      <Stack.Screen options={{ title: member.displayName }} />
      {/* オーナーのロールは、ここでは変えられない（交代は移譲で行う） */}
      {role !== 'owner' && (
        <View style={{ gap: theme.space.sm }}>
          <AppText variant="subheadline" color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
            {t('household.members.role')}
          </AppText>
          <View>
            {(['editor', 'viewer'] as const).map((r) => (
              <ListRow
                key={r}
                title={t(`roles.${r}`)}
                subtitle={t(`roles.description.${r}`)}
                selected={role === r}
                disabled={busy}
                onPress={role === r ? undefined : () => updateRole.mutate({ userId: member.userId, role: r })}
              />
            ))}
          </View>
        </View>
      )}
      <View>
        <ListRow title={t('household.transfer.action')} disabled={busy} onPress={() => void transferOwnership()} />
        <ListRow
          title={t('household.removeMember.action')}
          destructive
          disabled={busy}
          onPress={() => void removeMember()}
        />
      </View>
      {error && (
        <AppText variant="footnote" color="danger" style={{ paddingHorizontal: theme.space.lg }}>
          {t(errorMessageKey(error))}
        </AppText>
      )}
    </ScrollView>
  );
}
