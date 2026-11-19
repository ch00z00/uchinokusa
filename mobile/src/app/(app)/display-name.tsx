import { router } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, TextInput, View } from 'react-native';

import { ApiError } from '@/api';
import { errorMessageKey } from '@/api/errorMessage';
import { useProfileSetup } from '@/auth/AuthProvider';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { validateDisplayName, type DisplayNameError } from '@/features/account/displayName';
import { useMe, useUpdateDisplayName } from '@/features/account/useMe';
import { homeHref } from '@/features/invitation/pendingInvite';
import { useTheme } from '@/theme';

// AUTH-02 表示名の設定（初回）
export default function DisplayNameScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const me = useMe();

  // 初期値（プロバイダの名前）が届いてから入力欄を出す
  if (me.data) return <DisplayNameForm initialName={me.data.displayName} />;
  return (
    <View style={{ padding: theme.space.lg, gap: theme.space.md }}>
      {me.isError ? (
        <>
          <AppText color="secondary">{t(errorMessageKey(me.error))}</AppText>
          <Button title={t('common.retry')} variant="secondary" onPress={() => void me.refetch()} />
        </>
      ) : (
        <ActivityIndicator color={theme.color.text.secondary} />
      )}
    </View>
  );
}

function DisplayNameForm({ initialName }: { initialName: string }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const { completeProfileSetup } = useProfileSetup();
  const update = useUpdateDisplayName();
  const [name, setName] = useState(initialName);
  const [inputError, setInputError] = useState<DisplayNameError | null>(null);

  const submit = () => {
    const result = validateDisplayName(name);
    if (!result.ok) {
      setInputError(result.error);
      return;
    }
    setInputError(null);
    update.mutate(result.value, {
      onSuccess: () => {
        completeProfileSetup();
        // 招待リンクから来た場合は、その参加の確認へ（要件3-4）
        router.replace(homeHref());
      },
    });
  };

  // サーバーの入力のエラー（422 の fields.displayName）も、入力欄の下に出す
  const serverFieldError =
    update.error instanceof ApiError && update.error.code === 'validation_failed'
      ? update.error.fields.displayName
      : undefined;
  const fieldError =
    inputError ?? (serverFieldError === 'required' || serverFieldError === 'too_long' ? serverFieldError : null);

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      contentContainerStyle={{ padding: theme.space.lg, gap: theme.space.xl }}
    >
      <AppText variant="callout" color="secondary">
        {t('displayName.lead')}
      </AppText>
      <View style={{ gap: theme.space.sm }}>
        <TextInput
          value={name}
          onChangeText={(value) => {
            setName(value);
            setInputError(null);
          }}
          accessibilityLabel={t('displayName.label')}
          placeholder={t('displayName.label')}
          placeholderTextColor={theme.color.text.tertiary}
          autoFocus
          returnKeyType="done"
          onSubmitEditing={submit}
          style={[
            theme.text.body,
            {
              color: theme.color.text.primary,
              backgroundColor: theme.color.bg.subtle,
              borderRadius: theme.radius.md,
              minHeight: theme.size.tapMin,
              paddingHorizontal: theme.space.md,
            },
          ]}
        />
        {fieldError && (
          <AppText variant="footnote" color="danger">
            {t(`displayName.errors.${fieldError}`)}
          </AppText>
        )}
        {update.isError && !fieldError && (
          <AppText variant="footnote" color="danger">
            {t(errorMessageKey(update.error))}
          </AppText>
        )}
      </View>
      <Button title={t('displayName.submit')} onPress={submit} loading={update.isPending} />
    </ScrollView>
  );
}
