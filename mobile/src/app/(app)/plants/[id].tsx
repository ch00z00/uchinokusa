import { useLocalSearchParams } from 'expo-router';
import { ScrollView, Text } from 'react-native';

// PLANT-10 植物の詳細。中身はタスク 16・18 で作る
export default function PlantDetailScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic">
      <Text>{id}</Text>
    </ScrollView>
  );
}
