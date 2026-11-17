import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { ListRow } from '@/components/ListRow';
import { RoleBadge } from '@/components/RoleBadge';
import { useMe } from '@/features/account/useMe';
import { HouseholdForm } from '@/features/household/HouseholdForm';
import { leaveDecision } from '@/features/household/leave';
import { can } from '@/features/household/permissions';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import {
  useDeleteHousehold,
  useMembers,
  useRemoveMember,
  useUpdateHousehold,
  type Household,
} from '@/features/household/useHouseholds';
import { confirm, notify } from '@/lib/confirm';
import { useTheme } from '@/theme';

// HH-01 世帯の設定（選択中の世帯）。設定のトップ（SET-01）から開く
export default function HouseholdSettingsScreen() {
  const { state } = useSelectedHousehold();
  // 世帯を抜けたあとは、(app) のレイアウトがほかの世帯かはじめての選択に移す
  if (state.status !== 'selected') return null;
  // 世帯が変わったら、入力を初期化する
  return <HouseholdSettings key={state.household.id} household={state.household} />;
}

function HouseholdSettings({ household }: { household: Household }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const me = useMe();
  const members = useMembers(household.id);
  const update = useUpdateHousehold(household.id);
  const remove = useRemoveMember(household.id);
  const deleteHousehold = useDeleteHousehold(household.id);
  const isOwner = can(household.role, 'manageHousehold');

  const leave = async () => {
    if (!members.data || !me.data) return;
    const decision = leaveDecision(household.role, members.data.length);
    if (decision === 'transferRequired') {
      // 要件2-8：唯一のオーナーは、ほかにメンバーがいれば退出できない。先にオーナー権限を移す
      const go = await confirm({
        title: t('household.leave.transferRequiredTitle'),
        message: t('household.leave.transferRequiredMessage'),
        confirmLabel: t('household.transfer.title'),
        cancelLabel: t('common.cancel'),
      });
      if (go) router.push('/household/transfer');
      return;
    }
    if (decision === 'deleteRequired') {
      notify(t('household.leave.deleteRequiredTitle'), t('household.leave.deleteRequiredMessage'));
      return;
    }
    const ok = await confirm({
      title: t('household.leave.confirmTitle', { name: household.name }),
      message: t('household.leave.confirmMessage'),
      confirmLabel: t('household.leave.action'),
      cancelLabel: t('common.cancel'),
      destructive: true,
    });
    if (ok) remove.mutate({ userId: me.data.id, self: true }, { onSuccess: () => router.dismissTo('/today') });
  };

  const destroy = async () => {
    const ok = await confirm({
      title: t('household.delete.confirmTitle', { name: household.name }),
      message: t('household.delete.confirmMessage'),
      confirmLabel: t('household.delete.action'),
      cancelLabel: t('common.cancel'),
      destructive: true,
    });
    if (ok) deleteHousehold.mutate(undefined, { onSuccess: () => router.dismissTo('/today') });
  };

  const actionError = remove.error ?? deleteHousehold.error;

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.xl }}
    >
      <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.sm }}>
        <RoleBadge role={household.role} />
        {isOwner ? (
          <HouseholdForm
            initial={{
              name: household.name,
              regionPrefecture: household.regionPrefecture ?? '',
              regionCity: household.regionCity ?? '',
            }}
            submitLabel={t('common.save')}
            pending={update.isPending}
            error={update.error}
            // 地域は空文字を送ると未設定に戻る
            onSubmit={(value) => update.mutate(value)}
          />
        ) : (
          // 世帯の情報の変更はオーナーだけ（要件2-5、4-2）。ほかのロールには見せるだけ
          <View style={{ gap: theme.space.xs }}>
            <AppText variant="title3">{household.name}</AppText>
            <AppText color="secondary">
              {[household.regionPrefecture, household.regionCity].filter(Boolean).join(' ') ||
                t('household.form.regionUnset')}
            </AppText>
          </View>
        )}
        {update.isSuccess && (
          <AppText variant="footnote" color="secondary">
            {t('common.saved')}
          </AppText>
        )}
      </View>

      <View>
        <ListRow
          title={t('household.members.title')}
          detail={members.data ? String(members.data.length) : undefined}
          navigates
          onPress={() => router.push('/household/members')}
        />
        {/* TODO(15.2)：招待（INV-01 / INV-03）、TODO(16.1)：置き場所（LOC-01）への入口 */}
      </View>

      <View>
        {members.isPending && <ActivityIndicator color={theme.color.text.secondary} />}
        <ListRow
          title={t('household.leave.action')}
          destructive
          disabled={!members.data || !me.data || remove.isPending}
          onPress={() => void leave()}
        />
        {isOwner && (
          <ListRow
            title={t('household.delete.action')}
            destructive
            disabled={deleteHousehold.isPending}
            onPress={() => void destroy()}
          />
        )}
        {actionError && (
          <AppText variant="footnote" color="danger" style={{ padding: theme.space.lg }}>
            {t(errorMessageKey(actionError))}
          </AppText>
        )}
      </View>
    </ScrollView>
  );
}
