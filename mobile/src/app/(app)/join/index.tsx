import { router } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';

import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { FormField } from '@/components/FormField';
import { normalizeInviteCode, type InviteCodeError } from '@/features/invitation/inviteCode';
import { useTheme } from '@/theme';

// INV-10 招待コードの入力。はじめての選択（ONB-01）と、世帯の切り替え（HH-10）の「招待から参加する」で開く。
// QR コードの読み取り（INV-11）への入口も置く
export default function JoinScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const [code, setCode] = useState('');
  const [error, setError] = useState<InviteCodeError | null>(null);

  const submit = () => {
    const result = normalizeInviteCode(code);
    if (!result.ok) {
      setError(result.error);
      return;
    }
    // 招待が有効かは、参加の確認（INV-12）で API に問い合わせる
    router.push(`/invite/${result.value}`);
  };

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      contentContainerStyle={{ padding: theme.space.lg, gap: theme.space.xl }}
    >
      <AppText variant="callout" color="secondary">
        {t('join.lead')}
      </AppText>
      <View style={{ gap: theme.space.md }}>
        <FormField
          label={t('join.codeLabel')}
          placeholder={t('join.codePlaceholder')}
          value={code}
          onChangeText={(value) => {
            setCode(value);
            setError(null);
          }}
          error={error && t(`join.errors.${error}`)}
          autoCapitalize="characters"
          autoCorrect={false}
          autoComplete="off"
          returnKeyType="next"
          onSubmitEditing={submit}
        />
        <Button title={t('join.next')} onPress={submit} />
      </View>
      <Button title={t('join.scan')} variant="secondary" onPress={() => router.push('/join/scan')} />
    </ScrollView>
  );
}
