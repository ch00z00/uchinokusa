import { SegmentedControl } from '@expo/ui/community/segmented-control';
import { useTranslation } from 'react-i18next';
import { StyleSheet, TextInput, View } from 'react-native';

import { intervalUnits } from '@/features/task/careTask';
import type { IntervalForm } from '@/features/task/taskValidation';
import { useTheme } from '@/theme';

import { AppText } from './AppText';

type Props = {
  label: string;
  value: IntervalForm;
  onChange: (value: IntervalForm) => void;
  error?: string | null;
};

/**
 * 周期の入力（foundations.md の 5章の IntervalInput）。数（入力欄。数字のキーボード）＋単位（OS のセグメントの切り替え）＋「ごと」。
 * 入力欄は FormField と同じ見た目
 */
export function IntervalInput({ label, value, onChange, error }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  return (
    <View style={{ gap: theme.space.xs }}>
      <AppText variant="subheadline" color="secondary">
        {label}
      </AppText>
      <View style={[styles.row, { gap: theme.space.sm }]}>
        <TextInput
          accessibilityLabel={`${label} ${t('tasks.form.intervalCount')}`}
          value={value.value}
          onChangeText={(text) => onChange({ ...value, value: text })}
          keyboardType="number-pad"
          returnKeyType="done"
          maxLength={3}
          style={[
            theme.text.body,
            styles.count,
            {
              color: theme.color.text.primary,
              backgroundColor: theme.color.bg.subtle,
              borderRadius: theme.radius.md,
              minHeight: theme.size.tapMin,
              paddingHorizontal: theme.space.md,
              borderWidth: theme.borderWidth.hairline,
              borderColor: error ? theme.color.danger : 'transparent',
            },
          ]}
        />
        <SegmentedControl
          accessibilityLabel={`${label} ${t('tasks.form.intervalUnit')}`}
          values={intervalUnits.map((u) => t(`care.unit.${u}`))}
          selectedIndex={intervalUnits.indexOf(value.unit)}
          onChange={(e) => {
            const unit = intervalUnits[e.nativeEvent.selectedSegmentIndex];
            if (unit) onChange({ ...value, unit });
          }}
          style={styles.units}
        />
        <AppText>{t('tasks.form.every')}</AppText>
      </View>
      {error && (
        <AppText variant="footnote" color="danger">
          {error}
        </AppText>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
  count: { width: 72, textAlign: 'center' },
  units: { flex: 1 },
});
