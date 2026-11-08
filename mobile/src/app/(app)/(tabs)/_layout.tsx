import { NativeTabs } from 'expo-router/native-tabs';
import { useTranslation } from 'react-i18next';

import { useTheme } from '@/theme';

// タブバーは OS の層（.kiro/steering/design.md）なので、OS の標準のタブバーを使う。
// アイコンは docs/design/foundations.md の 4.3（iOS：SF Symbols、Android：Material Symbols）
export default function TabsLayout() {
  const { t } = useTranslation();
  const theme = useTheme();
  return (
    // 選択中のタブはアクセントの色（foundations.md の color/accent）
    <NativeTabs tintColor={theme.color.accent.default}>
      <NativeTabs.Trigger name="today">
        <NativeTabs.Trigger.Icon sf="checklist" md="checklist" />
        <NativeTabs.Trigger.Label>{t('tabs.today')}</NativeTabs.Trigger.Label>
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="plants">
        <NativeTabs.Trigger.Icon sf="leaf" md="potted_plant" />
        <NativeTabs.Trigger.Label>{t('tabs.plants')}</NativeTabs.Trigger.Label>
      </NativeTabs.Trigger>
    </NativeTabs>
  );
}
