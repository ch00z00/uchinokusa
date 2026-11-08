import { Link } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { Pressable } from 'react-native';

import { useTheme } from '@/theme';

import { AppText } from './AppText';

// 各タブのヘッダーの右に置く、設定を開くボタン。
// TODO：アイコン（SF Symbols の gearshape / Material Symbols の settings。foundations.md の 4.3）にする
export function SettingsHeaderButton() {
  const { t } = useTranslation();
  const theme = useTheme();
  return (
    <Link href="/settings" asChild>
      <Pressable accessibilityRole="button" hitSlop={theme.space.md}>
        <AppText color="accent">{t('settings.open')}</AppText>
      </Pressable>
    </Link>
  );
}
