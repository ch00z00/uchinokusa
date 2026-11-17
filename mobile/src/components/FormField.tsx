import { TextInput, View, type TextInputProps } from 'react-native';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

export type FormFieldProps = Omit<TextInputProps, 'style' | 'placeholderTextColor'> & {
  label: string;
  /** 入力欄の下に出す理由（screen-inventory.md の FormField のエラー） */
  error?: string | null;
};

/** 入力欄（アプリの層。screen-inventory.md の FormField）。見出し・入力欄・エラーの理由を縦に並べる */
export function FormField({ label, error, ...props }: FormFieldProps) {
  const theme = useTheme();
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
        ]}
        {...props}
      />
      {error && (
        <AppText variant="footnote" color="danger">
          {error}
        </AppText>
      )}
    </View>
  );
}
