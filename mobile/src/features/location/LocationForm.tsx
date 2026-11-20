import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Keyboard, View } from 'react-native';

import { ApiError } from '@/api';
import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { ChoiceField } from '@/components/ChoiceField';
import { FormField } from '@/components/FormField';
import { useTheme } from '@/theme';

import {
  shadings,
  validateLocationForm,
  windowDirections,
  windowDistances,
  type LocationFieldError,
  type LocationFormInput,
} from './locationValidation';

type Props = {
  initial: LocationFormInput;
  submitLabel: string;
  onSubmit: (value: LocationFormInput) => void;
  pending: boolean;
  /** 送信の失敗（422 の入力のエラーは、入力欄の下に出す） */
  error: unknown;
};

/** 置き場所名と環境の入力（LOC-02 の作成と編集で使う） */
export function LocationForm({ initial, submitLabel, onSubmit, pending, error }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  const [input, setInput] = useState(initial);
  const [nameError, setNameError] = useState<LocationFieldError | null>(null);

  const change = <K extends keyof LocationFormInput>(field: K, value: LocationFormInput[K]) => {
    setInput((v) => ({ ...v, [field]: value }));
    // 入力を直したら、エラーを消す（直したのに赤いままにしない）
    if (field === 'name') setNameError(null);
  };

  const submit = () => {
    const result = validateLocationForm(input);
    if (!result.ok) {
      setNameError(result.errors.name ?? null);
      return;
    }
    // 送ったら入力欄のフォーカスを外す（キーボードを閉じ、結果を見えるようにする）
    Keyboard.dismiss();
    onSubmit(result.value);
  };

  const serverFields = error instanceof ApiError && error.code === 'validation_failed' ? error.fields : {};
  const nameCode = nameError ?? serverFields.name;
  const nameMessage =
    nameCode === undefined || nameCode === null
      ? null
      : nameCode === 'required' || nameCode === 'too_long'
        ? t(`location.form.errors.name.${nameCode}`)
        : t('errors.validation_failed');
  const hasFieldError = nameError !== null || Object.keys(serverFields).length > 0;

  // 「未設定」を先頭に置く
  const unset = { value: '', label: t('location.form.unset') };

  return (
    <View style={{ gap: theme.space.lg }}>
      <FormField
        label={t('location.form.name')}
        placeholder={t('location.form.namePlaceholder')}
        value={input.name}
        onChangeText={(name) => change('name', name)}
        error={nameMessage}
        returnKeyType="done"
      />
      <AppText variant="footnote" color="secondary">
        {t('location.form.environmentNote')}
      </AppText>
      <ChoiceField
        label={t('location.form.indoor')}
        options={[
          { value: 'true', label: t('location.indoor.true') },
          { value: 'false', label: t('location.indoor.false') },
        ]}
        value={input.isIndoor ? 'true' : 'false'}
        onChange={(v) => change('isIndoor', v === 'true')}
      />
      <ChoiceField
        // 屋外では窓がないので、その場所が面している向きとして聞く（API の WindowDirection の説明）
        label={t(input.isIndoor ? 'location.form.windowDirection' : 'location.form.direction')}
        options={[unset, ...windowDirections.map((v) => ({ value: v, label: t(`location.windowDirection.${v}`) }))]}
        value={input.windowDirection}
        onChange={(v) => change('windowDirection', v as LocationFormInput['windowDirection'])}
      />
      {/* 窓からの距離と遮光は、室内だけ（屋外にしたら、保存のときに未設定に戻す） */}
      {input.isIndoor && (
        <>
          <ChoiceField
            label={t('location.form.windowDistance')}
            options={[unset, ...windowDistances.map((v) => ({ value: v, label: t(`location.windowDistance.${v}`) }))]}
            value={input.windowDistance}
            onChange={(v) => change('windowDistance', v as LocationFormInput['windowDistance'])}
          />
          <ChoiceField
            label={t('location.form.shading')}
            options={[unset, ...shadings.map((v) => ({ value: v, label: t(`location.shading.${v}`) }))]}
            value={input.shading}
            onChange={(v) => change('shading', v as LocationFormInput['shading'])}
          />
        </>
      )}
      {error !== null && error !== undefined && !hasFieldError && (
        <AppText variant="footnote" color="danger">
          {t(errorMessageKey(error))}
        </AppText>
      )}
      <Button title={submitLabel} onPress={submit} loading={pending} />
    </View>
  );
}
