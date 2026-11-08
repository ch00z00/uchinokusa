import { useLocalSearchParams } from 'expo-router';
import { ScrollView } from 'react-native';

import { AppText } from '@/components/AppText';
import { useTheme } from '@/theme';

// INV-12 参加の確認。招待リンク https://<招待用ドメイン>/i/{token} から開く。中身はタスク 15.2 で作る
export default function InviteScreen() {
  const theme = useTheme();
  const { token } = useLocalSearchParams<{ token: string }>();
  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={{ padding: theme.space.lg }}>
      <AppText>{token}</AppText>
    </ScrollView>
  );
}
