import { Link } from 'expo-router';
import type { NativeStackNavigationOptions } from 'expo-router/build/react-navigation/native-stack/types';
import { useTranslation } from 'react-i18next';
import { Pressable } from 'react-native';

import { AppText } from '@/components/AppText';
import { useHeaderHitArea } from '@/components/HeaderTextButton';

import { useSelectedHousehold } from './SelectedHouseholdProvider';

/**
 * タブの画面のヘッダーの左に置く、選択中の世帯名（screen-inventory.md の HouseholdSwitcher）。
 * 押すと世帯の切り替え（HH-10）を開く。世帯が1つだけのときも出す
 */
export function HouseholdSwitcher() {
  const { t } = useTranslation();
  const hitArea = useHeaderHitArea();
  const { state } = useSelectedHousehold();
  if (state.status !== 'selected') return null;
  return (
    <Link href="/households/switch" asChild>
      <Pressable
        accessibilityRole="button"
        accessibilityHint={t('household.switcher.hint')}
        // 押せる領域は 44pt 以上（ネイティブのヘッダーでは hitSlop が効かない。useHeaderHitArea）。
        // Link の asChild にはスタイルの配列を渡せないので、1つのオブジェクトにする
        style={{ ...hitArea, maxWidth: 200, alignItems: 'flex-start' }}
      >
        {/* TODO：「▾」を OS のシンボル（chevron.down / expand_more）にする（SettingsHeaderButton と同じく、アイコンの導入のあと） */}
        <AppText variant="headline" color="accent" numberOfLines={1}>
          {state.household.name} ▾
        </AppText>
      </Pressable>
    </Link>
  );
}

/**
 * タブの画面のヘッダーの左に世帯名を出す設定。
 * iOS 26 はヘッダーのボタンにガラスの背景を付けるが、世帯名は見出しの一部として見せたいので、背景を消す
 */
export const householdSwitcherOptions: NativeStackNavigationOptions = {
  // Android と iOS 26 より前
  headerLeft: () => <HouseholdSwitcher />,
  // iOS では headerLeft の代わりにこちらが使われる
  unstable_headerLeftItems: () => [{ type: 'custom', element: <HouseholdSwitcher />, hidesSharedBackground: true }],
};
