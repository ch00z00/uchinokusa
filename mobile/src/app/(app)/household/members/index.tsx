import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { ListRow } from '@/components/ListRow';
import { RoleBadge } from '@/components/RoleBadge';
import { useMe } from '@/features/account/useMe';
import { can } from '@/features/household/permissions';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { useMembers, type Household } from '@/features/household/useHouseholds';
import { useTheme } from '@/theme';

// HH-02 メンバーの一覧。オーナーは、自分以外のメンバーを押すと、ロールの変更・削除・移譲ができる
export default function MembersScreen() {
  const { state } = useSelectedHousehold();
  if (state.status !== 'selected') return null;
  return <Members household={state.household} />;
}

function Members({ household }: { household: Household }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const me = useMe();
  const members = useMembers(household.id);
  const isOwner = can(household.role, 'manageHousehold');

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.lg }}
    >
      {members.isPending && <ActivityIndicator color={theme.color.text.secondary} />}
      {members.isError && (
        <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.md }}>
          <AppText color="secondary">{t(errorMessageKey(members.error))}</AppText>
          <Button title={t('common.retry')} variant="secondary" onPress={() => void members.refetch()} />
        </View>
      )}
      <View>
        {members.data?.map((member) => {
          const self = member.userId === me.data?.id;
          return (
            <ListRow
              key={member.userId}
              title={self ? t('household.members.self', { name: member.displayName }) : member.displayName}
              accessory={<RoleBadge role={member.role} />}
              navigates={isOwner && !self}
              onPress={
                isOwner && !self
                  ? () => router.push({ pathname: '/household/members/[userId]', params: { userId: member.userId } })
                  : undefined
              }
            />
          );
        })}
      </View>
      <AppText variant="footnote" color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
        {t('household.members.rolesHelp')}
      </AppText>
    </ScrollView>
  );
}
