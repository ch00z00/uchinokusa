import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Keyboard, ScrollView, View } from 'react-native';

import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { DateField } from '@/components/DateField';
import { FormField } from '@/components/FormField';
import { ListRow } from '@/components/ListRow';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { taskTitle } from '@/features/task/taskText';
import { emptySkipForm, skipReasons, validateSkip, type SkipField, type SkipFieldError } from '@/features/today/skip';
import type { TodayItem } from '@/features/today/todayList';
import { useCompletionFlow } from '@/features/today/useCompletionFlow';
import { TodayItemMissing } from '@/features/today/TodayItemMissing';
import { useOpenedTodayItem } from '@/features/today/useOpenedTodayItem';
import { toDayString } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// TODAY-03 スキップ（OS のシート。要件10-6〜10-8）。今日の一覧のカードの長押しのメニューから開く
export default function SkipScreen() {
  const { taskId } = useLocalSearchParams<{ taskId: string }>();
  const { state } = useSelectedHousehold();
  const householdId = state.status === 'selected' ? state.household.id : '';
  const { item, loading } = useOpenedTodayItem(householdId, taskId);
  if (!item) return <TodayItemMissing loading={loading} />;
  return <SkipForm householdId={householdId} item={item} />;
}

function SkipForm({ householdId, item }: { householdId: string; item: TodayItem }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const flow = useCompletionFlow(householdId);
  const tomorrowDate = new Date();
  tomorrowDate.setDate(tomorrowDate.getDate() + 1);
  const tomorrow = toDayString(tomorrowDate);
  const [form, setForm] = useState(() => emptySkipForm(tomorrow));
  const [errors, setErrors] = useState<Partial<Record<SkipField, SkipFieldError>>>({});

  const submit = async () => {
    const found = validateSkip(form, tomorrow);
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    Keyboard.dismiss();
    if (await flow.skip(item, form)) router.back();
  };

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="interactive"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.lg }}
    >
      <AppText color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
        {t('today.skip.lead', { plant: item.plant.name, task: taskTitle(item.task, t) })}
      </AppText>
      <View style={{ gap: theme.space.xs }}>
        <AppText variant="subheadline" color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
          {t('today.skip.reason')}
        </AppText>
        <View>
          {skipReasons.map((reason) => (
            <ListRow
              key={reason}
              title={t(`today.skip.reasons.${reason}`)}
              selected={form.reason === reason}
              // もう一度押すと、選んだ理由を外す（理由は任意）
              onPress={() => setForm({ ...form, reason: form.reason === reason ? '' : reason })}
            />
          ))}
        </View>
      </View>
      <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.lg }}>
        {form.reason === 'other' && (
          <FormField
            label={t('today.skip.reasonText')}
            placeholder={t('today.skip.reasonTextPlaceholder')}
            value={form.reasonText}
            onChangeText={(reasonText) => setForm({ ...form, reasonText })}
            error={errors.reasonText ? t(`today.skip.errors.reasonText.${errors.reasonText as 'too_long'}`) : null}
            returnKeyType="done"
          />
        )}
        <DateField
          label={t('today.skip.recheckOn')}
          value={form.recheckOn}
          onChange={(recheckOn) => setForm({ ...form, recheckOn })}
          minimumDate={tomorrow}
          clearable={false}
          error={errors.recheckOn ? t(`today.skip.errors.recheckOn.${errors.recheckOn as 'past'}`) : null}
        />
        <Button title={t('today.skip.submit')} onPress={() => void submit()} loading={flow.pending} />
      </View>
    </ScrollView>
  );
}
