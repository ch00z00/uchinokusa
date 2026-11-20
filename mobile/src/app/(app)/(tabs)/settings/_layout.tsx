import { Stack } from 'expo-router';
import { useTranslation } from 'react-i18next';

// 設定のタブ（SET-01）。世帯の設定（HH-01）・メンバー・招待・置き場所も、このタブの中で進む（戻るボタンに「設定」と出る）
export default function SettingsLayout() {
  const { t } = useTranslation();
  return (
    <Stack>
      <Stack.Screen name="index" options={{ title: t('settings.title'), headerLargeTitleEnabled: true }} />
      <Stack.Screen name="household/index" options={{ title: t('household.settings.title') }} />
      <Stack.Screen name="household/members/index" options={{ title: t('household.members.title') }} />
      <Stack.Screen name="household/members/[userId]" options={{ title: '' }} />
      <Stack.Screen name="household/transfer" options={{ title: t('household.transfer.title') }} />
      <Stack.Screen name="household/invitations/index" options={{ title: t('invitations.title') }} />
      <Stack.Screen name="household/locations" options={{ title: t('location.list.title') }} />
      {/* INV-01 で作り、同じモーダルのまま INV-02 に切り替える */}
      <Stack.Screen
        name="household/invitations/new"
        options={{ title: t('invitations.create.title'), presentation: 'modal' }}
      />
    </Stack>
  );
}
