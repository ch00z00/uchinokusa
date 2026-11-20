import { useTranslation } from 'react-i18next';

import { ChoiceField } from '@/components/ChoiceField';

import { prefectures } from './prefectures';

type Props = {
  /** 選んだ都道府県。空文字は未設定 */
  value: string;
  onChange: (value: string) => void;
};

/** 都道府県を一覧から選ぶ欄。押すと、OS のシートで「未設定」と47の一覧を出す */
export function PrefectureField({ value, onChange }: Props) {
  const { t } = useTranslation();
  return (
    <ChoiceField
      label={t('household.form.prefecture')}
      options={[{ value: '', label: t('household.form.unset') }, ...prefectures.map((p) => ({ value: p, label: p }))]}
      value={value}
      onChange={onChange}
      accessibilityHint={t('household.form.prefectureHint')}
    />
  );
}
