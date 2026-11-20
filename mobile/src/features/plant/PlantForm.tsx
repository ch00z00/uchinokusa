import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Keyboard, View } from 'react-native';

import { ApiError } from '@/api';
import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { ChoiceField } from '@/components/ChoiceField';
import { DateField } from '@/components/DateField';
import { FormField } from '@/components/FormField';
import { ListRow } from '@/components/ListRow';
import { openNewLocation } from '@/features/location/newLocationFlow';
import { useLocations } from '@/features/location/useLocations';
import { toDayString } from '@/lib/formatDate';
import { useTheme } from '@/theme';

import { validatePlantForm, type PlantFieldError, type PlantFormInput } from './plantValidation';
import { openSpeciesPicker } from './speciesPicker';

type Props = {
  householdId: string;
  initial: PlantFormInput;
  submitLabel: string;
  onSubmit: (value: PlantFormInput) => void;
  pending: boolean;
  /** 送信の失敗（422 の入力のエラーは、入力欄の下に出す） */
  error: unknown;
};

// サーバーの入力のエラーの項目名を、入力欄にあてはめる
const serverFieldOf: Record<string, keyof PlantFormInput> = {
  name: 'name',
  speciesId: 'species',
  speciesFreeText: 'species',
  locationId: 'locationId',
  acquiredOn: 'acquiredOn',
};

/** 植物の名前・種類・置き場所・購入日の入力（PLANT-02 の登録と編集で使う） */
export function PlantForm({ householdId, initial, submitLabel, onSubmit, pending, error }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  const locations = useLocations(householdId);
  const [input, setInput] = useState(initial);
  const [inputErrors, setInputErrors] = useState<Partial<Record<keyof PlantFormInput, PlantFieldError>>>({});
  const today = toDayString(new Date());

  // 入力を直したら、その欄のエラーを消す（直したのに赤いままにしない）
  const change = <K extends keyof PlantFormInput>(field: K, value: PlantFormInput[K]) => {
    setInput((v) => ({ ...v, [field]: value }));
    setInputErrors(({ [field]: _, ...rest }) => rest);
  };

  const submit = () => {
    const result = validatePlantForm(input, today);
    if (!result.ok) {
      setInputErrors(result.errors);
      return;
    }
    setInputErrors({});
    // 送ったら入力欄のフォーカスを外す（キーボードを閉じ、結果を見えるようにする）
    Keyboard.dismiss();
    onSubmit(result.value);
  };

  const serverErrors: Partial<Record<keyof PlantFormInput, string>> = {};
  if (error instanceof ApiError && error.code === 'validation_failed') {
    for (const [field, code] of Object.entries(error.fields)) {
      const key = serverFieldOf[field];
      if (key) serverErrors[key] = code;
    }
  }
  const fieldError = (field: keyof PlantFormInput) => {
    const code = inputErrors[field] ?? serverErrors[field];
    if (code === undefined) return null;
    if (field === 'name' && (code === 'required' || code === 'too_long')) return t(`plants.form.errors.name.${code}`);
    if (field === 'species' && code === 'too_long') return t('plants.form.errors.species.too_long');
    if (field === 'acquiredOn' && code === 'future') return t('plants.form.errors.acquiredOn.future');
    return t('errors.validation_failed');
  };
  const hasFieldError = Object.keys(inputErrors).length > 0 || Object.keys(serverErrors).length > 0;

  const species = input.species;
  const speciesLabel =
    species.kind === 'species' ? species.name : species.kind === 'free' ? species.text : t('plants.form.unset');

  return (
    <View style={{ gap: theme.space.lg }}>
      <FormField
        label={t('plants.form.name')}
        placeholder={t('plants.form.namePlaceholder')}
        value={input.name}
        onChangeText={(name) => change('name', name)}
        error={fieldError('name')}
        returnKeyType="done"
      />
      <AppText variant="footnote" color="secondary">
        {t('plants.form.optionalNote')}
      </AppText>
      {/* 種類：植物マスタから選ぶか、自由入力（PLANT-03 のシート） */}
      <View style={{ gap: theme.space.xs }}>
        <AppText variant="subheadline" color="secondary">
          {t('plants.form.species')}
        </AppText>
        <View style={{ borderRadius: theme.radius.md, overflow: 'hidden' }}>
          <ListRow
            title={speciesLabel}
            subtitle={species.kind === 'free' ? t('plants.form.speciesFree') : undefined}
            navigates
            accessibilityHint={t('plants.form.speciesHint')}
            onPress={() => openSpeciesPicker(input.species, (value) => change('species', value))}
          />
        </View>
        {fieldError('species') && (
          <AppText variant="footnote" color="danger">
            {fieldError('species')}
          </AppText>
        )}
      </View>
      <ChoiceField
        label={t('plants.form.location')}
        options={[
          { value: '', label: t('plants.form.unset') },
          ...(locations.data ?? []).map((l) => ({ value: l.id, label: l.name })),
        ]}
        value={input.locationId}
        onChange={(locationId) => change('locationId', locationId)}
        // その場で新しい置き場所を作り、そのまま選ぶ（2026-11-20）
        action={{
          label: t('plants.form.newLocation'),
          onPress: () =>
            openNewLocation({ replace: true, onCreated: (locationId) => change('locationId', locationId) }),
        }}
      />
      <DateField
        label={t('plants.form.acquiredOn')}
        value={input.acquiredOn}
        onChange={(acquiredOn) => change('acquiredOn', acquiredOn)}
        // 未来の日は選べない（植え替えの提案の起点になるため）
        maximumDate={today}
        error={fieldError('acquiredOn')}
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
