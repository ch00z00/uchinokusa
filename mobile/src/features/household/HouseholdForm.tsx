import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { ApiError } from '@/api';
import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { FormField } from '@/components/FormField';
import { useTheme } from '@/theme';

import { validateHouseholdForm, type HouseholdFieldError, type HouseholdFormInput } from './householdValidation';
import { PrefectureField } from './PrefectureField';

type Props = {
  initial: HouseholdFormInput;
  submitLabel: string;
  onSubmit: (value: HouseholdFormInput) => void;
  pending: boolean;
  /** 送信の失敗（422 の入力のエラーは、入力欄の下に出す） */
  error: unknown;
};

/** 世帯名・地域の入力（ONB-02 の作成と、HH-01 の変更で使う） */
export function HouseholdForm({ initial, submitLabel, onSubmit, pending, error }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  const [input, setInput] = useState(initial);
  const [inputErrors, setInputErrors] = useState<Partial<Record<keyof HouseholdFormInput, HouseholdFieldError>>>({});

  // 入力を直したら、その欄のエラーを消す（直したのに赤いままにしない）
  const change = <K extends keyof HouseholdFormInput>(field: K, value: HouseholdFormInput[K]) => {
    setInput((v) => ({ ...v, [field]: value }));
    setInputErrors(({ [field]: _, ...rest }) => rest);
  };

  const submit = () => {
    const result = validateHouseholdForm(input);
    if (!result.ok) {
      setInputErrors(result.errors);
      return;
    }
    setInputErrors({});
    onSubmit(result.value);
  };

  const serverFields = error instanceof ApiError && error.code === 'validation_failed' ? error.fields : {};
  const fieldError = (field: keyof HouseholdFormInput) => {
    const code = inputErrors[field] ?? serverFields[field];
    if (code === undefined) return null;
    return code === 'required' || code === 'too_long'
      ? t(`household.form.errors.${field === 'name' ? 'name' : 'region'}.${code}`)
      : t('errors.validation_failed');
  };
  const hasFieldError = Object.keys(inputErrors).length > 0 || Object.keys(serverFields).length > 0;

  return (
    <View style={{ gap: theme.space.lg }}>
      <FormField
        label={t('household.form.name')}
        placeholder={t('household.form.namePlaceholder')}
        value={input.name}
        onChangeText={(name) => change('name', name)}
        error={fieldError('name')}
        returnKeyType="done"
      />
      <AppText variant="footnote" color="secondary">
        {t('household.form.regionNote')}
      </AppText>
      <PrefectureField
        value={input.regionPrefecture}
        onChange={(regionPrefecture) => change('regionPrefecture', regionPrefecture)}
      />
      <FormField
        label={t('household.form.city')}
        placeholder={t('household.form.cityPlaceholder')}
        value={input.regionCity}
        onChangeText={(regionCity) => change('regionCity', regionCity)}
        error={fieldError('regionCity')}
      />
      {error !== null && error !== undefined && !hasFieldError && (
        <AppText variant="footnote" color="danger">
          {t(errorMessageKey(error))}
        </AppText>
      )}
      <Button title={submitLabel} onPress={submit} loading={pending} />
    </View>
  );
}
