import { Link } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { Pressable, Text } from 'react-native';

// 各タブのヘッダーの右に置く、設定を開くボタン。
// アイコン（SF Symbols の gearshape など）はタスク 14.3 以降でテーマとあわせて入れる
export function SettingsHeaderButton() {
  const { t } = useTranslation();
  return (
    <Link href="/settings" asChild>
      <Pressable accessibilityRole="button" hitSlop={12}>
        <Text>{t('settings.open')}</Text>
      </Pressable>
    </Link>
  );
}
