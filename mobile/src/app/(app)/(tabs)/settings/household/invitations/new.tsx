import { router } from 'expo-router';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { ScrollView, Share, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { ListRow } from '@/components/ListRow';
import { QRCodeView } from '@/components/QRCodeView';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { useCreateInvitation, type CreatedInvitation, type InvitableRole } from '@/features/invitation/useInvitations';
import { formatDate } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// INV-01 招待の作成 → INV-02 招待の共有（要件3-1、3-2）。
// 招待リンクの URL は作成のときにしか受け取れないので、作ったら同じモーダルのまま共有の表示に切り替える
export default function NewInvitationScreen() {
  const { state } = useSelectedHousehold();
  if (state.status !== 'selected') return null;
  return <NewInvitation householdId={state.household.id} householdName={state.household.name} />;
}

function NewInvitation({ householdId, householdName }: { householdId: string; householdName: string }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const create = useCreateInvitation(householdId);
  const [role, setRole] = useState<InvitableRole>('editor');

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.xl }}
    >
      {create.data ? (
        <ShareInvitation invitation={create.data} householdName={householdName} />
      ) : (
        <>
          <AppText variant="callout" color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
            {t('invitations.create.lead')}
          </AppText>
          <View>
            {(['editor', 'viewer'] as const).map((r) => (
              <ListRow
                key={r}
                title={t(`roles.${r}`)}
                subtitle={t(`roles.description.${r}`)}
                selected={role === r}
                onPress={() => setRole(r)}
              />
            ))}
          </View>
          <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.md }}>
            <AppText variant="footnote" color="secondary">
              {t('invitations.create.expiresNote')}
            </AppText>
            <Button
              title={t('invitations.create.submit')}
              loading={create.isPending}
              onPress={() => create.mutate(role)}
            />
            {create.isError && (
              <AppText variant="footnote" color="danger">
                {t(errorMessageKey(create.error))}
              </AppText>
            )}
          </View>
        </>
      )}
    </ScrollView>
  );
}

function ShareInvitation({ invitation, householdName }: { invitation: CreatedInvitation; householdName: string }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const roleLabel = t(`roles.${invitation.role === 'editor' ? 'editor' : 'viewer'}`);

  // OS の共有シート（LINE、メッセージなど）
  const share = () =>
    void Share.share({
      message: t('invitations.share.message', { household: householdName, url: invitation.url, code: invitation.code }),
    });

  const section = (title: string, note: string, children: ReactNode) => (
    <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.sm }}>
      <AppText variant="headline">{title}</AppText>
      <AppText variant="footnote" color="secondary">
        {note}
      </AppText>
      {children}
    </View>
  );

  return (
    <>
      <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.xs }}>
        <AppText variant="callout">{t('invitations.share.lead', { role: roleLabel })}</AppText>
        <AppText variant="footnote" color="secondary">
          {t('invitations.share.expires', { date: formatDate(invitation.expiresAt) })}
        </AppText>
      </View>
      {section(
        t('invitations.share.link'),
        t('invitations.share.linkNote'),
        <Button title={t('invitations.share.link')} onPress={share} />,
      )}
      {section(
        t('invitations.share.qr'),
        t('invitations.share.qrNote'),
        <QRCodeView value={invitation.url} accessibilityLabel={t('invitations.share.qr')} />,
      )}
      {section(
        t('invitations.share.code'),
        t('invitations.share.codeNote'),
        <AppText variant="largeTitle" selectable style={{ letterSpacing: theme.space.xs, textAlign: 'center' }}>
          {invitation.code}
        </AppText>,
      )}
      <View style={{ paddingHorizontal: theme.space.lg }}>
        <Button title={t('invitations.share.done')} variant="secondary" onPress={() => router.back()} />
      </View>
    </>
  );
}
