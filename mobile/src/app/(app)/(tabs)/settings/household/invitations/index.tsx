import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { ListRow } from '@/components/ListRow';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { useInvitations, useRevokeInvitation, type Invitation } from '@/features/invitation/useInvitations';
import { confirm } from '@/lib/confirm';
import { formatDate } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// INV-03 招待の一覧と取り消し（要件3-7）。オーナーだけが、世帯の設定（HH-01）から開く
export default function InvitationsScreen() {
  const { state } = useSelectedHousehold();
  if (state.status !== 'selected') return null;
  return <Invitations householdId={state.household.id} />;
}

function Invitations({ householdId }: { householdId: string }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const invitations = useInvitations(householdId);
  const revoke = useRevokeInvitation(householdId);

  const confirmRevoke = async (invitation: Invitation) => {
    const ok = await confirm({
      title: t('invitations.revoke.title'),
      message: t('invitations.revoke.message', { code: invitation.code }),
      confirmLabel: t('invitations.revoke.confirm'),
      cancelLabel: t('common.cancel'),
      destructive: true,
    });
    if (ok) revoke.mutate(invitation.id);
  };

  let list;
  if (invitations.data) {
    list =
      invitations.data.length === 0 ? (
        <AppText color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
          {t('invitations.empty')}
        </AppText>
      ) : (
        <View>
          {invitations.data.map((invitation) => (
            <ListRow
              key={invitation.id}
              title={invitation.code}
              subtitle={t('invitations.item', {
                role: t(`roles.${invitation.role === 'editor' ? 'editor' : 'viewer'}`),
                date: formatDate(invitation.expiresAt),
              })}
              trailing={
                <Button
                  title={t('invitations.revoke.confirm')}
                  variant="secondary"
                  disabled={revoke.isPending}
                  accessibilityHint={t('invitations.revoke.hint', { code: invitation.code })}
                  onPress={() => void confirmRevoke(invitation)}
                />
              }
            />
          ))}
        </View>
      );
  } else if (invitations.isError) {
    list = (
      <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.md }}>
        <AppText color="secondary">{t(errorMessageKey(invitations.error))}</AppText>
        <Button title={t('common.retry')} variant="secondary" onPress={() => void invitations.refetch()} />
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
      <View style={{ paddingHorizontal: theme.space.lg }}>
        <Button title={t('invitations.new')} onPress={() => router.push('/settings/household/invitations/new')} />
      </View>
      {list}
      {revoke.isError && (
        <AppText variant="footnote" color="danger" style={{ paddingHorizontal: theme.space.lg }}>
          {t(errorMessageKey(revoke.error))}
        </AppText>
      )}
      <AppText variant="footnote" color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
        {t('invitations.listNote')}
      </AppText>
    </ScrollView>
  );
}
