import { Text, type TextProps } from 'react-native';

import { useTheme, type TextVariant } from '@/theme';

export type AppTextColor = 'primary' | 'secondary' | 'tertiary' | 'accent' | 'onAccent' | 'danger';

export type AppTextProps = TextProps & {
  /** 文字スタイル（foundations.md の 2章）。既定は body */
  variant?: TextVariant;
  /** 文字の色（color/text/* など）。既定は primary */
  color?: AppTextColor;
};

/** トークンの文字スタイルと色で表示する Text。画面では React Native の Text の代わりにこれを使う */
export function AppText({ variant = 'body', color = 'primary', style, ...props }: AppTextProps) {
  const theme = useTheme();
  const colorValue = {
    primary: theme.color.text.primary,
    secondary: theme.color.text.secondary,
    tertiary: theme.color.text.tertiary,
    accent: theme.color.accent.default,
    onAccent: theme.color.onAccent,
    danger: theme.color.danger,
  }[color];

  return <Text style={[theme.text[variant], { color: colorValue }, style]} {...props} />;
}
