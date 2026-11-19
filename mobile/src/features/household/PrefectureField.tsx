import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/components/AppText';
import { ListRow } from '@/components/ListRow';
import { useTheme } from '@/theme';

import { openPrefecturePicker } from './prefecturePicker';

type Props = {
  /** 選んだ都道府県。空文字は未設定 */
  value: string;
  onChange: (value: string) => void;
};

/** 都道府県を一覧から選ぶ欄。押すと、OS のシート（/prefecture）で47の一覧と「未設定」を出す */
export function PrefectureField({ value, onChange }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();

  return (
    <View style={{ gap: theme.space.xs }}>
      <AppText variant="subheadline" color="secondary">
        {t('household.form.prefecture')}
      </AppText>
      <View style={{ borderRadius: theme.radius.md, overflow: 'hidden' }}>
        <ListRow
          title={value || t('household.form.unset')}
          navigates
          onPress={() => openPrefecturePicker(value, onChange)}
          accessibilityHint={t('household.form.prefectureHint')}
        />
      </View>
    </View>
  );
}
