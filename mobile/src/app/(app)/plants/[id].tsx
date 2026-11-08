import { useLocalSearchParams } from 'expo-router';
import { ScrollView } from 'react-native';

import { AppText } from '@/components/AppText';
import { useTheme } from '@/theme';

// PLANT-10 植物の詳細。中身はタスク 16・18 で作る
export default function PlantDetailScreen() {
  const theme = useTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  return (
    <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={{ padding: theme.space.lg }}>
      <AppText>{id}</AppText>
    </ScrollView>
  );
}
