import { useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Button, StyleSheet, Text, View } from 'react-native';

import { authClient } from '@/auth/authClient';

// AUTH-01 サインイン。Apple / Google のサインインはタスク 14.2 で作る
export default function SignInScreen() {
  const { t } = useTranslation();
  const devSignIn = useMutation({
    mutationFn: async () => authClient.signInForDevelopment?.(),
  });

  return (
    <View style={styles.container}>
      <Text>{t('auth.signIn.title')}</Text>
      <Text>{t('auth.signIn.lead')}</Text>
      {authClient.signInForDevelopment && (
        <Button
          title={t('auth.signIn.development')}
          onPress={() => devSignIn.mutate()}
          disabled={devSignIn.isPending}
        />
      )}
    </View>
  );
}

// 見た目はタスク 14.3 でテーマのトークンを当てる
const styles = StyleSheet.create({
  container: { flex: 1, alignItems: 'center', justifyContent: 'center' },
});
