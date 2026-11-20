import { DateTimePicker } from '@expo/ui/community/datetime-picker';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Platform, StyleSheet, View } from 'react-native';

import { formatDay, parseDay, toDayString } from '@/lib/formatDate';
import { useTheme } from '@/theme';

import { AppText } from './AppText';
import { Button } from './Button';
import { ListRow } from './ListRow';

type Props = {
  label: string;
  /** `YYYY-MM-DD`。空文字は未設定 */
  value: string;
  onChange: (value: string) => void;
  /** 選べる最後の日（`YYYY-MM-DD`）。購入日なら今日 */
  maximumDate?: string;
  /** 入力欄の下に出す理由 */
  error?: string | null;
  /** 「未設定に戻す」を出すか（任意の項目なら true） */
  clearable?: boolean;
};

/**
 * 日付の欄（foundations.md の 5章の DateField）。日付の選択は OS の部品（iOS：compact の DatePicker、Android：日付のダイアログ）。
 * 任意の項目なので、「未設定」から始められ、未設定に戻せる
 */
export function DateField({ label, value, onChange, maximumDate, error, clearable = true }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  // Android のダイアログを出しているか
  const [dialogOpen, setDialogOpen] = useState(false);
  const max = maximumDate ? parseDay(maximumDate) : undefined;
  const today = toDayString(new Date());
  const pick = (date: Date) => onChange(toDayString(date));

  const ios = Platform.OS === 'ios';
  return (
    <View style={{ gap: theme.space.xs }}>
      <AppText variant="subheadline" color="secondary">
        {label}
      </AppText>
      <View style={{ borderRadius: theme.radius.md, overflow: 'hidden' }}>
        {value !== '' && ios ? (
          // iOS は欄の中に compact の DatePicker を置く（押すと OS のカレンダーが開く）
          <View
            style={[
              styles.row,
              {
                minHeight: theme.size.tapMin,
                paddingHorizontal: theme.space.lg,
                paddingVertical: theme.space.xs,
                backgroundColor: theme.color.bg.surface,
              },
            ]}
          >
            <DateTimePicker
              value={parseDay(value)}
              mode="date"
              display="compact"
              maximumDate={max}
              accentColor={theme.color.accent.default}
              onValueChange={(_, date) => pick(date)}
            />
          </View>
        ) : (
          <ListRow
            title={value === '' ? t('dateField.unset') : formatDay(value)}
            navigates
            accessibilityHint={t('dateField.hint')}
            // iOS は今日（選べる最後の日のほうが前ならその日）を入れて、compact の DatePicker に切り替える
            onPress={() =>
              ios ? onChange(maximumDate && maximumDate < today ? maximumDate : today) : setDialogOpen(true)
            }
          />
        )}
      </View>
      {dialogOpen && (
        <DateTimePicker
          value={value === '' ? new Date() : parseDay(value)}
          mode="date"
          maximumDate={max}
          accentColor={theme.color.accent.default}
          onValueChange={(_, date) => {
            setDialogOpen(false);
            pick(date);
          }}
          onDismiss={() => setDialogOpen(false)}
        />
      )}
      {clearable && value !== '' && (
        <View style={styles.clear}>
          <Button title={t('dateField.clear')} variant="secondary" onPress={() => onChange('')} />
        </View>
      )}
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
  clear: { alignSelf: 'flex-start' },
});
