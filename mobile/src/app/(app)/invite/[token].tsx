import { router, useLocalSearchParams } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { ApiError } from '@/api';
import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { RoleBadge } from '@/components/RoleBadge';
import { normalizeRole } from '@/features/household/permissions';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { clearPendingInvite } from '@/features/invitation/pendingInvite';
import { useAcceptInvitation, useInvitationPreview } from '@/features/invitation/useInvitations';
import { formatDate } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// INV-12 参加の確認（要件3-3〜3-6）。招待リンク（/i/<トークン> を読み替える）、
// 招待コードの入力（INV-10）、QR コードの読み取り（INV-11）から開く。token はトークンかコード
export default function InviteScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { token } = useLocalSearchParams<{ token: string }>();
  const preview = useInvitationPreview(token);
  const accept = useAcceptInvitation(token);
  const { select } = useSelectedHousehold();

  const openToday = () => {
    clearPendingInvite();
    router.dismissTo('/today');
  };

  const join = () =>
    accept.mutate(undefined, {
      onSuccess: (household) => {
        clearPendingInvite();
        // 参加した世帯に切り替えて「今日」へ
        select(household.id);
        router.dismissTo('/today');
      },
    });

  // 参加を押したあとに無効・参加済みとわかった場合も、同じ状態として出す
  const failure = preview.error ?? accept.error;
  const invalid = failure instanceof ApiError && failure.code === 'invitation_invalid';
  const alreadyMember =
    preview.data?.alreadyMember === true ||
    (accept.error instanceof ApiError && accept.error.code === 'already_member');

  let body;
  if (invalid || alreadyMember) {
    // 要件3-5（無効）、3-6（参加済み）
    // TODO：参加済みのときは、その世帯に切り替えて開く（世帯の ID を API に依頼中。HANDOFF-mobile.md の依頼2）
    body = (
      <>
        <AppText variant="title3">{t(invalid ? 'invite.invalid' : 'invite.alreadyMember')}</AppText>
        <Button title={t('invite.openToday')} variant="secondary" onPress={openToday} />
      </>
    );
  } else if (preview.data) {
    const role = normalizeRole(preview.data.role);
    body = (
      <>
        <AppText variant="title2">{t('invite.lead', { household: preview.data.householdName })}</AppText>
        <View style={{ gap: theme.space.sm }}>
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: theme.space.sm }}>
            <AppText variant="subheadline" color="secondary">
              {t('invite.role')}
            </AppText>
            <RoleBadge role={role} />
          </View>
          {role !== 'owner' && <AppText color="secondary">{t(`roles.description.${role}`)}</AppText>}
          <AppText variant="footnote" color="secondary">
            {t('invite.expires', { date: formatDate(preview.data.expiresAt) })}
          </AppText>
        </View>
        <Button title={t('invite.accept')} onPress={join} loading={accept.isPending} />
        {accept.isError && (
          <AppText variant="footnote" color="danger">
            {t(errorMessageKey(accept.error))}
          </AppText>
        )}
      </>
    );
  } else if (preview.isError) {
    body = (
      <>
        <AppText color="secondary">{t(errorMessageKey(preview.error))}</AppText>
        <Button title={t('common.retry')} variant="secondary" onPress={() => void preview.refetch()} />
      </>
    );
  } else {
    body = <ActivityIndicator color={theme.color.text.secondary} />;
  }

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ padding: theme.space.lg, gap: theme.space.xl }}
    >
      {body}
    </ScrollView>
  );
}
