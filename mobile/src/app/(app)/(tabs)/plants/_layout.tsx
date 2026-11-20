import { Link, Stack } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { Pressable } from 'react-native';

import { AppText } from '@/components/AppText';
import { householdSwitcherOptions } from '@/features/household/HouseholdSwitcher';
import { useTheme } from '@/theme';

export default function PlantsLayout() {
  const { t } = useTranslation();
  return (
    <Stack>
      <Stack.Screen
        name="index"
        options={{
          title: t('plants.title'),
          headerLargeTitleEnabled: true,
          // 選択中の世帯名。押すと世帯の切り替え（HH-10）。設定は、タブバーの「設定」から開く
          ...householdSwitcherOptions,
          // 置き場所の管理（LOC-01）。世帯名と同じく、ヘッダーのボタンの背景は付けない
          headerRight: () => <LocationsLink />,
          unstable_headerRightItems: () => [
            { type: 'custom', element: <LocationsLink />, hidesSharedBackground: true },
          ],
        }}
      />
      <Stack.Screen name="locations" options={{ title: t('location.list.title') }} />
    </Stack>
  );
}

function LocationsLink() {
  const { t } = useTranslation();
  const theme = useTheme();
  return (
    <Link href="/plants/locations" asChild>
      <Pressable accessibilityRole="button" hitSlop={theme.space.md}>
        <AppText variant="headline" color="accent">
          {t('location.list.title')}
        </AppText>
      </Pressable>
    </Link>
  );
}
