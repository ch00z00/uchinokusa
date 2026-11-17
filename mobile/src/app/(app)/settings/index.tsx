import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { useSignOut } from '@/auth/useSignOut';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { useMe } from '@/features/account/useMe';
import { useTheme } from '@/theme';

// SET-01 設定のトップ。中身はタスク 14.2・15・19 で作る
export default function SettingsScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const me = useMe();
  const signOut = useSignOut();

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ padding: theme.space.lg, gap: theme.space.md }}
    >
      {me.isPending && <ActivityIndicator color={theme.color.text.secondary} />}
      {me.isError && (
        <>
          <AppText color="secondary">{t(errorMessageKey(me.error))}</AppText>
          <Button title={t('common.retry')} variant="secondary" onPress={() => void me.refetch()} />
        </>
      )}
      {me.data && <AppText variant="headline">{me.data.displayName}</AppText>}
      <Button
        title={t('settings.signOut')}
        variant="secondary"
        loading={signOut.isPending}
        onPress={() => signOut.mutate()}
      />
    </ScrollView>
  );
}
