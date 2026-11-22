import { useTranslation } from 'react-i18next';
import { StyleSheet, Switch, View } from 'react-native';

import { seasons, type Season } from '@/features/task/careTask';
import type { IntervalForm, TaskFormInput } from '@/features/task/taskValidation';
import { useTheme } from '@/theme';

import { AppText } from './AppText';
import { IntervalInput } from './IntervalInput';

type Props = {
  value: TaskFormInput['seasonal'];
  onChange: (value: TaskFormInput['seasonal']) => void;
  /** 季節ごとの入力の誤り */
  errorOf: (season: Season) => string | null;
};

/**
 * 季節ごとの周期（foundations.md の 5章の SeasonalIntervalEditor。要件8-6）。
 * ふだんは閉じておき、OS のスイッチ「季節ごとに変える」で、春・夏・秋・冬の周期の入力を開く
 */
export function SeasonalIntervalEditor({ value, onChange, errorOf }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  const set = (season: Season, interval: IntervalForm) => onChange({ ...value, [season]: interval });
  return (
    <View style={{ gap: theme.space.md }}>
      <View style={[styles.row, { gap: theme.space.md, minHeight: theme.size.tapMin }]}>
        <View style={[styles.text, { gap: theme.space.xs }]}>
          <AppText>{t('tasks.form.seasonal')}</AppText>
          <AppText variant="footnote" color="secondary">
            {t('tasks.form.seasonalHint')}
          </AppText>
        </View>
        <Switch
          accessibilityLabel={t('tasks.form.seasonal')}
          value={value.enabled}
          onValueChange={(enabled) => onChange({ ...value, enabled })}
          trackColor={{ true: theme.color.accent.default }}
        />
      </View>
      {value.enabled &&
        seasons.map((season) => (
          <IntervalInput
            key={season}
            label={t(`care.seasonMonths.${season}`)}
            value={value[season]}
            onChange={(interval) => set(season, interval)}
            error={errorOf(season)}
          />
        ))}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
  text: { flex: 1 },
});
