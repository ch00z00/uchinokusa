import { Pressable } from 'react-native';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

type Props = {
  title: string;
  onPress: () => void;
  accessibilityHint?: string;
};

/**
 * ヘッダーに置く文字のボタン（「追加」「編集」など）。世帯名（HouseholdSwitcher）と同じ見た目で、背景は付けない。
 * iOS 26 のガラスの背景を消すには、headerTextButtonOptions で置く
 */
export function HeaderTextButton({ title, onPress, accessibilityHint }: Props) {
  const theme = useTheme();
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityHint={accessibilityHint}
      hitSlop={theme.space.md}
      onPress={onPress}
    >
      <AppText variant="headline" color="accent">
        {title}
      </AppText>
    </Pressable>
  );
}

/** ヘッダーの右に文字のボタンを置く設定（iOS 26 のヘッダーのボタンのガラスの背景を付けない） */
export function headerRightTextButton(props: Props) {
  return {
    // Android と iOS 26 より前
    headerRight: () => <HeaderTextButton {...props} />,
    // iOS では headerRight の代わりにこちらが使われる
    unstable_headerRightItems: () => [
      { type: 'custom' as const, element: <HeaderTextButton {...props} />, hidesSharedBackground: true },
    ],
  };
}
