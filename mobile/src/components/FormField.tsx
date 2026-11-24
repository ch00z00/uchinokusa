import { TextInput, View, type TextInputProps } from 'react-native';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

export type FormFieldProps = Omit<TextInputProps, 'style' | 'placeholderTextColor'> & {
  label: string;
  /** 入力欄の下に出す理由（screen-inventory.md の FormField のエラー） */
  error?: string | null;
  /** 複数行の入力欄にし、この行数ぶんの高さを最初からとる（入力に合わせて伸びる）。メモなど */
  rows?: number;
  /** 入力欄の下の右に出す文字数（「123 / 2000」）。上限を超えたら danger の色にする */
  count?: { length: number; max: number };
};

/** 入力欄（アプリの層。screen-inventory.md の FormField）。見出し・入力欄・エラーの理由を縦に並べる */
export function FormField({ label, error, rows, count, ...props }: FormFieldProps) {
  const theme = useTheme();
  const over = count !== undefined && count.length > count.max;
  return (
    <View style={{ gap: theme.space.xs }}>
      <AppText variant="subheadline" color="secondary">
        {label}
      </AppText>
      <TextInput
        accessibilityLabel={label}
        placeholderTextColor={theme.color.text.tertiary}
        style={[
          theme.text.body,
          {
            color: theme.color.text.primary,
            backgroundColor: theme.color.bg.subtle,
            borderRadius: theme.radius.md,
            minHeight: theme.size.tapMin,
            paddingHorizontal: theme.space.md,
            borderWidth: theme.borderWidth.hairline,
            borderColor: error ? theme.color.danger : 'transparent',
          },
          rows !== undefined && {
            minHeight: rows * theme.text.body.lineHeight + theme.space.md * 2,
            paddingVertical: theme.space.md,
            textAlignVertical: 'top',
          },
        ]}
        multiline={rows !== undefined || props.multiline}
        {...props}
      />
      {(error || count) && (
        <View style={{ flexDirection: 'row', gap: theme.space.md }}>
          <AppText variant="footnote" color="danger" style={{ flex: 1 }}>
            {error ?? ''}
          </AppText>
          {count && (
            <AppText variant="footnote" color={over ? 'danger' : 'secondary'}>
              {`${count.length} / ${count.max}`}
            </AppText>
          )}
        </View>
      )}
    </View>
  );
}
