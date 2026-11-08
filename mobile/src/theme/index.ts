import { DarkTheme, DefaultTheme, type Theme as NavigationTheme } from 'expo-router';
import { useColorScheme } from 'react-native';

import { borderWidth, colors, radius, size, space, text, type ColorTokens } from './tokens';

export type { TextVariant } from './tokens';

export type ColorScheme = 'light' | 'dark';

export type Theme = {
  scheme: ColorScheme;
  color: ColorTokens;
  text: typeof text;
  space: typeof space;
  radius: typeof radius;
  size: typeof size;
  borderWidth: typeof borderWidth;
};

const themes: Record<ColorScheme, Theme> = {
  light: { scheme: 'light', color: colors.light, text, space, radius, size, borderWidth },
  dark: { scheme: 'dark', color: colors.dark, text, space, radius, size, borderWidth },
};

/** 端末のライト・ダークの設定に合わせたテーマ。色・余白などは、生の値ではなくここから使う */
export function useTheme(): Theme {
  return themes[useColorScheme() === 'dark' ? 'dark' : 'light'];
}

/**
 * 画面遷移（ヘッダー・画面の背景・タブ）の色を、トークンから作る。
 * ナビゲーションバーなどの OS の層の部品は、OS の見た目のまま、色だけをそろえる
 */
export function toNavigationTheme(theme: Theme): NavigationTheme {
  const base = theme.scheme === 'dark' ? DarkTheme : DefaultTheme;
  return {
    ...base,
    colors: {
      ...base.colors,
      primary: theme.color.accent.default,
      background: theme.color.bg.base,
      card: theme.color.bg.base,
      text: theme.color.text.primary,
      border: theme.color.border.subtle,
      notification: theme.color.accent.default,
    },
  };
}
