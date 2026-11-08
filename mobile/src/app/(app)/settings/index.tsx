import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Button, ScrollView, Text } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { authClient } from '@/auth/authClient';
import { useMe } from '@/features/account/useMe';

// SET-01 設定のトップ。中身はタスク 14.2・15・19 で作る
export default function SettingsScreen() {
  const { t } = useTranslation();
  const me = useMe();

  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic">
      {me.isPending && <ActivityIndicator />}
      {me.isError && (
        <>
          <Text>{t(errorMessageKey(me.error))}</Text>
          <Button title={t('common.retry')} onPress={() => void me.refetch()} />
        </>
      )}
      {me.data && <Text>{me.data.displayName}</Text>}
      {/* TODO(14.2)：端末の情報の破棄と、通知の送信先からの削除をあわせて行う */}
      <Button title={t('settings.signOut')} onPress={() => void authClient.signOut()} />
    </ScrollView>
  );
}
