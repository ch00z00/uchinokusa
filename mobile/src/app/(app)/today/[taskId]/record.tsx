import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ScrollView } from 'react-native';

import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { DateField } from '@/components/DateField';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { taskTitle } from '@/features/task/taskText';
import type { TodayItem } from '@/features/today/todayList';
import { useCompletionFlow } from '@/features/today/useCompletionFlow';
import { TodayItemMissing } from '@/features/today/TodayItemMissing';
import { useOpenedTodayItem } from '@/features/today/useOpenedTodayItem';
import { toDayString } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// TODAY-04 実施日を変えて記録（OS のシート。要件10-3）。今日の一覧のカードの長押しのメニューから開く
export default function RecordScreen() {
  const { taskId } = useLocalSearchParams<{ taskId: string }>();
  const { state } = useSelectedHousehold();
  const householdId = state.status === 'selected' ? state.household.id : '';
  const { item, loading } = useOpenedTodayItem(householdId, taskId);
  if (!item) return <TodayItemMissing loading={loading} />;
  return <RecordForm householdId={householdId} item={item} />;
}

function RecordForm({ householdId, item }: { householdId: string; item: TodayItem }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const flow = useCompletionFlow(householdId);
  const todayString = toDayString(new Date());
  const [performedOn, setPerformedOn] = useState(todayString);

  const submit = async () => {
    // 今日なら実施日を送らない（サーバーが世帯のタイムゾーンでの今日にする）
    if (await flow.complete(item, performedOn === todayString ? {} : { performedOn })) router.back();
  };

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ padding: theme.space.lg, gap: theme.space.lg }}
    >
      <AppText color="secondary">
        {t('today.record.lead', { plant: item.plant.name, task: taskTitle(item.task, t) })}
      </AppText>
      {/* 未来の日は選べない（要件10-3） */}
      <DateField
        label={t('today.record.performedOn')}
        value={performedOn}
        onChange={setPerformedOn}
        maximumDate={todayString}
        clearable={false}
      />
      <Button title={t('today.record.submit')} onPress={() => void submit()} loading={flow.pending} />
    </ScrollView>
  );
}
