import { View } from 'react-native';

import { openChoicePicker, type Choice } from '@/lib/choicePicker';
import { useTheme } from '@/theme';

import { AppText } from './AppText';
import { ListRow } from './ListRow';

type Props = {
  label: string;
  options: readonly Choice[];
  value: string;
  onChange: (value: string) => void;
  /** 読み上げで、押すと何が開くかを伝える */
  accessibilityHint?: string;
};

/**
 * 決まった選択肢から1つを選ぶ欄（アプリの層。foundations.md の 5章の ChoiceField）。
 * 押すと OS のシート（/choice の ChoiceSheet）で選択肢の一覧を出す。都道府県や、置き場所の環境の項目で使う
 */
export function ChoiceField({ label, options, value, onChange, accessibilityHint }: Props) {
  const theme = useTheme();
  const current = options.find((o) => o.value === value) ?? options[0];
  return (
    <View style={{ gap: theme.space.xs }}>
      <AppText variant="subheadline" color="secondary">
        {label}
      </AppText>
      <View style={{ borderRadius: theme.radius.md, overflow: 'hidden' }}>
        <ListRow
          title={current?.label ?? ''}
          navigates
          accessibilityHint={accessibilityHint}
          onPress={() => openChoicePicker({ title: label, options, value, onChange })}
        />
      </View>
    </View>
  );
}
