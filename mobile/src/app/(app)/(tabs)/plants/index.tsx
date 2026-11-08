import { Link } from 'expo-router';
import { ScrollView } from 'react-native';

import { AppText } from '@/components/AppText';
import { useTheme } from '@/theme';

// PLANT-01 植物の一覧。中身はタスク 16 で作る
export default function PlantsScreen() {
  const theme = useTheme();
  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={{ padding: theme.space.lg }}>
      {/* TODO(16)：画面遷移の確認用の仮のリンク。一覧を作ったら消す */}
      {__DEV__ && (
        <Link href={{ pathname: '/plants/[id]', params: { id: 'sample' } }}>
          <AppText color="accent">/plants/sample</AppText>
        </Link>
      )}
    </ScrollView>
  );
}
