import { NativeTabs } from 'expo-router/native-tabs';
import { useTranslation } from 'react-i18next';
import { Linking, Platform } from 'react-native';

import { useToast } from '@/components/Toast';
import { openGardenShopSearch } from '@/features/shop/gardenShop';

import { useTheme } from '@/theme';

// タブバーは OS の層（.kiro/steering/design.md）なので、OS の標準のタブバーを使う。
// アイコンは docs/design/foundations.md の 4.3（iOS：SF Symbols、Android：Material Symbols）
export default function TabsLayout() {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  // 「探す」：画面を持たず、押すと地図アプリで近くの園芸店を検索する（19.1、要件18）
  const searchShops = async () => {
    if (!(await openGardenShopSearch(Platform.OS, t('shops.query'), (url) => Linking.openURL(url)))) {
      toast.show(t('shops.failed'));
    }
  };
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
      {/* disabled にすると、OS はこのタブへ切り替えず、押されたことだけを知らせる（tabPress）。そこで地図アプリを開く */}
      <NativeTabs.Trigger
        name="shops"
        disabled
        accessibilityLabel={t('shops.a11yLabel')}
        listeners={{ tabPress: () => void searchShops() }}
      >
        <NativeTabs.Trigger.Icon sf="map" md="map" />
        <NativeTabs.Trigger.Label>{t('tabs.shops')}</NativeTabs.Trigger.Label>
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="settings">
        <NativeTabs.Trigger.Icon sf="gearshape" md="settings" />
        <NativeTabs.Trigger.Label>{t('tabs.settings')}</NativeTabs.Trigger.Label>
      </NativeTabs.Trigger>
    </NativeTabs>
  );
}
