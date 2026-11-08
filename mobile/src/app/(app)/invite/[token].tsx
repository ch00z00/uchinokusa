import { useLocalSearchParams } from 'expo-router';
import { ScrollView, Text } from 'react-native';

// INV-12 参加の確認。招待リンク https://<招待用ドメイン>/i/{token} から開く。中身はタスク 15.2 で作る
export default function InviteScreen() {
  const { token } = useLocalSearchParams<{ token: string }>();
  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic">
      <Text>{token}</Text>
    </ScrollView>
  );
}
