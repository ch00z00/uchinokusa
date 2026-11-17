import { Link } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { Pressable } from 'react-native';

import { AppText } from '@/components/AppText';
import { useTheme } from '@/theme';

import { useSelectedHousehold } from './SelectedHouseholdProvider';

/**
 * タブの画面のヘッダーの左に置く、選択中の世帯名（screen-inventory.md の HouseholdSwitcher）。
 * 押すと世帯の切り替え（HH-10）を開く。世帯が1つだけのときも出す
 */
export function HouseholdSwitcher() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { state } = useSelectedHousehold();
  if (state.status !== 'selected') return null;
  return (
    <Link href="/households/switch" asChild>
      <Pressable
        accessibilityRole="button"
        accessibilityHint={t('household.switcher.hint')}
        hitSlop={theme.space.md}
        style={{ maxWidth: 200 }}
      >
        {/* TODO：「▾」を OS のシンボル（chevron.down / expand_more）にする（SettingsHeaderButton と同じく、アイコンの導入のあと） */}
        <AppText variant="headline" color="accent" numberOfLines={1}>
          {state.household.name} ▾
        </AppText>
      </Pressable>
    </Link>
  );
}
