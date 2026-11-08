import { useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { authClient } from '@/auth/authClient';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { useTheme } from '@/theme';

// AUTH-01 サインイン。Apple / Google のサインインはタスク 14.2 で作る
export default function SignInScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const devSignIn = useMutation({
    mutationFn: async () => authClient.signInForDevelopment?.(),
  });

  return (
    <SafeAreaView style={[styles.container, { padding: theme.space.lg, gap: theme.space['2xl'] }]}>
      <View style={[styles.heading, { gap: theme.space.sm }]}>
        <AppText variant="largeTitle">{t('auth.signIn.title')}</AppText>
        <AppText variant="callout" color="secondary">
          {t('auth.signIn.lead')}
        </AppText>
      </View>
      {authClient.signInForDevelopment && (
        <Button
          title={t('auth.signIn.development')}
          variant="secondary"
          onPress={() => devSignIn.mutate()}
          loading={devSignIn.isPending}
        />
      )}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: 'center' },
  heading: { alignItems: 'center' },
});
