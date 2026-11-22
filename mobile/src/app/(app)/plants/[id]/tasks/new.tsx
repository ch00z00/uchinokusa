import { router, useLocalSearchParams } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { useToast } from '@/components/Toast';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { TaskForm } from '@/features/task/TaskForm';
import { emptyTaskForm, toCreateTaskRequest } from '@/features/task/taskValidation';
import { useCreateCareTask, useTaskDefaults } from '@/features/task/useCareTasks';
import { toDayString } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// TASK-01 お手入れタスクの作成（要件8-1〜8-6、6-4）。植物の詳細（PLANT-10）の「＋ お手入れを追加」から開くモーダル（編集者以上）
export default function NewTaskScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { state } = useSelectedHousehold();
  const householdId = state.status === 'selected' ? state.household.id : '';
  const defaults = useTaskDefaults(id);
  const create = useCreateCareTask(householdId, id);
  const today = toDayString(new Date());

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="interactive"
      contentContainerStyle={{ padding: theme.space.lg }}
    >
      {defaults.data ? (
        <TaskForm
          initial={emptyTaskForm(today)}
          defaults={defaults.data}
          creating
          dueLabel={t('tasks.form.firstDueOn')}
          // 初回の予定日は今日から（過去の日にすると、作った直後から期限切れになるため）
          minDueOn={today}
          submitLabel={t('tasks.create.submit')}
          pending={create.isPending}
          error={create.error}
          onSubmit={(value) =>
            create.mutate(toCreateTaskRequest(value), {
              onSuccess: () => {
                toast.show(t('tasks.create.done'));
                router.back();
              },
            })
          }
        />
      ) : defaults.isError ? (
        <View style={{ gap: theme.space.md }}>
          <AppText color="secondary">{t(errorMessageKey(defaults.error))}</AppText>
          <Button title={t('common.retry')} variant="secondary" onPress={() => void defaults.refetch()} />
        </View>
      ) : (
        <ActivityIndicator color={theme.color.text.secondary} />
      )}
    </ScrollView>
  );
}
