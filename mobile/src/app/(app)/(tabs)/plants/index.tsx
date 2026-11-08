import { Link } from 'expo-router';
import { ScrollView, Text } from 'react-native';

// PLANT-01 植物の一覧。中身はタスク 16 で作る
export default function PlantsScreen() {
  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic">
      {/* TODO(16)：画面遷移の確認用の仮のリンク。一覧を作ったら消す */}
      {__DEV__ && (
        <Link href={{ pathname: '/plants/[id]', params: { id: 'sample' } }}>
          <Text>/plants/sample</Text>
        </Link>
      )}
    </ScrollView>
  );
}
