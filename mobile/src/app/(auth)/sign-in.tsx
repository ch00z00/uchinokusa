import { useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useSignIn } from '@/auth/AuthProvider';
import type { AuthProviderId } from '@/auth/types';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { hasPendingInvite } from '@/features/invitation/pendingInvite';
import { useTheme } from '@/theme';

// AUTH-01 サインイン
export default function SignInScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const signIn = useSignIn();
  const mutation = useMutation({ mutationFn: (provider: AuthProviderId) => signIn(provider) });
  const pendingProvider = mutation.isPending ? mutation.variables : null;

  return (
    <SafeAreaView style={[styles.container, { padding: theme.space.lg, gap: theme.space['2xl'] }]}>
      <View style={[styles.heading, { gap: theme.space.sm }]}>
        <AppText variant="largeTitle">{t('auth.signIn.title')}</AppText>
        <AppText variant="callout" color="secondary">
          {t('auth.signIn.lead')}
        </AppText>
      </View>
      {/* 招待リンクから来た場合（AUTH-01、要件3-4） */}
      {hasPendingInvite() && (
        <AppText variant="callout" style={styles.center}>
          {t('auth.signIn.inviteNote')}
        </AppText>
      )}
      {/* Apple と Google は、同じ大きさ・同じ目立ち方にする（App Store の審査のルール）。
          Apple のガイドラインどおりのボタンにするのは、タスク 14.4 */}
      <View style={{ gap: theme.space.md }}>
        <Button
          title={t('auth.signIn.apple')}
          variant="secondary"
          loading={pendingProvider === 'apple'}
          disabled={mutation.isPending}
          onPress={() => mutation.mutate('apple')}
        />
        <Button
          title={t('auth.signIn.google')}
          variant="secondary"
          loading={pendingProvider === 'google'}
          disabled={mutation.isPending}
          onPress={() => mutation.mutate('google')}
        />
        {/* キャンセルは失敗にしない（SignInResult の cancelled）ので、ここには出ない */}
        {mutation.isError && (
          <AppText variant="footnote" color="danger" style={styles.center}>
            {t('auth.signIn.failed')}
          </AppText>
        )}
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: 'center' },
  heading: { alignItems: 'center' },
  center: { textAlign: 'center' },
});
