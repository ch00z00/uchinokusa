import { router } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppleReauthUnavailable } from '@/auth/types';
import { AppText } from '@/components/AppText';
import { ListRow } from '@/components/ListRow';
import { classifyDeleteError, DeleteCancelled, useDeleteAccount } from '@/features/account/deleteAccount';
import { confirm } from '@/lib/confirm';
import { useTheme } from '@/theme';

// SET-04 アカウントの削除（要件1-6・1-7、タスク 19.2）。設定（SET-01）のいちばん下から開く。閲覧者も使える
export default function AccountDeleteScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const remove = useDeleteAccount();
  const [appleUnavailable, setAppleUnavailable] = useState(false);
  const failure = remove.error && !(remove.error instanceof DeleteCancelled) ? classifyDeleteError(remove.error) : null;

  const deleteAccount = async () => {
    const ok = await confirm({
      title: t('account.delete.confirmTitle'),
      message: t('account.delete.confirmMessage'),
      confirmLabel: t('account.delete.confirm'),
      cancelLabel: t('common.cancel'),
      destructive: true,
    });
    if (!ok) return;
    setAppleUnavailable(false);
    // 成功すると、サインインの状態が消えて、サインインの画面へ移る（(app)/_layout.tsx）
    remove.mutate(undefined, {
      onError: (error) => {
        if (error instanceof AppleReauthUnavailable) setAppleUnavailable(true);
      },
    });
  };

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.xl }}
    >
      <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.sm }}>
        <AppText>{t('account.delete.lead')}</AppText>
        {(['account', 'households', 'ownData', 'sharedData'] as const).map((key) => (
          <AppText key={key} color="secondary">
            {`・${t(`account.delete.items.${key}`)}`}
          </AppText>
        ))}
        <AppText variant="headline">{t('account.delete.irreversible')}</AppText>
      </View>

      {failure?.kind === 'owner_transfer_required' && (
        // 唯一のオーナーの世帯（要件1-7）。押すと、その世帯のオーナー権限の移譲（HH-03）を開く
        <View style={{ gap: theme.space.sm }}>
          <AppText color="danger" style={{ paddingHorizontal: theme.space.lg }}>
            {t('account.delete.transferRequired')}
          </AppText>
          <View>
            {failure.households.map((h) => (
              <ListRow
                key={h.id}
                title={h.name}
                navigates
                onPress={() => router.push({ pathname: '/settings/household/transfer', params: { householdId: h.id } })}
              />
            ))}
          </View>
        </View>
      )}

      <View>
        <ListRow
          title={t('account.delete.action')}
          destructive
          disabled={remove.isPending}
          onPress={() => void deleteAccount()}
        />
        {(appleUnavailable || failure?.kind === 'apple_reauth' || failure?.kind === 'other') && (
          <AppText variant="footnote" color="danger" style={{ padding: theme.space.lg }}>
            {appleUnavailable
              ? t('account.delete.appleUnavailable')
              : failure?.kind === 'apple_reauth'
                ? t('account.delete.appleRetry')
                : failure?.kind === 'other'
                  ? t(errorMessageKey(failure.error))
                  : ''}
          </AppText>
        )}
      </View>
    </ScrollView>
  );
}
