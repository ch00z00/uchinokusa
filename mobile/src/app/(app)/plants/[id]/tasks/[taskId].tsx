import { router, useLocalSearchParams } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { ListRow } from '@/components/ListRow';
import { useToast } from '@/components/Toast';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import type { CareTask } from '@/features/task/careTask';
import { TaskForm } from '@/features/task/TaskForm';
import { taskTitle } from '@/features/task/taskText';
import { editMinDueOn, taskToForm, toUpdateTaskRequest } from '@/features/task/taskValidation';
import {
  useCareTasks,
  useDeleteCareTask,
  usePauseCareTask,
  useResumeCareTask,
  useTaskDefaults,
  useUpdateCareTask,
} from '@/features/task/useCareTasks';
import { useRepottingAvoidAlert } from '@/features/task/useRepottingAvoidAlert';
import { confirm } from '@/lib/confirm';
import { formatDay, toDayString } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// TASK-01 お手入れタスクの編集（要件8-8〜8-10）。植物の詳細（PLANT-10）のタスクの項目から開くモーダル（編集者以上）
// タスク1件を取る API はないので、植物のタスクの一覧から探す。下に、一時停止・再開と削除を置く
export default function EditTaskScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { id, taskId } = useLocalSearchParams<{ id: string; taskId: string }>();
  const { state } = useSelectedHousehold();
  const tasks = useCareTasks(id);
  const defaults = useTaskDefaults(id);
  const task = tasks.data?.find((candidate) => candidate.id === taskId);

  if (state.status !== 'selected') return null;
  if (task && defaults.data) {
    return <EditTask key={task.id} householdId={state.household.id} task={task} defaults={defaults.data} />;
  }
  const error = tasks.error ?? defaults.error;
  return (
    <View style={{ padding: theme.space.lg, gap: theme.space.md }}>
      {error ? (
        <>
          <AppText color="secondary">{t(errorMessageKey(error))}</AppText>
          <Button
            title={t('common.retry')}
            variant="secondary"
            onPress={() => void Promise.all([tasks.refetch(), defaults.refetch()])}
          />
        </>
      ) : tasks.data && !task ? (
        <AppText color="secondary">{t('tasks.edit.notFound')}</AppText>
      ) : (
        <ActivityIndicator color={theme.color.text.secondary} />
      )}
    </View>
  );
}

function EditTask({
  householdId,
  task,
  defaults,
}: {
  householdId: string;
  task: CareTask;
  defaults: Parameters<typeof TaskForm>[0]['defaults'];
}) {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  const update = useUpdateCareTask(householdId, task.plantId, task.id);
  const pause = usePauseCareTask(householdId, task.plantId, task.id);
  const resume = useResumeCareTask(householdId, task.plantId, task.id);
  const remove = useDeleteCareTask(householdId, task.plantId, task.id);
  const alertRepottingAvoid = useRepottingAvoidAlert(householdId);
  const initial = taskToForm(task);
  const paused = task.status === 'paused';
  const busy = pause.isPending || resume.isPending || remove.isPending;

  // 一時停止・再開・削除のあとは、モーダルを閉じて植物の詳細へ戻る（フォームの書きかけは保存しない）。
  // 成功するとタスクが変わり（削除なら一覧から消え）、この画面が描き直される。そのあいだも確実に戻れるように、
  // mutate の onSuccess ではなく、mutateAsync で完了を待つ。失敗は各 error で下に出す
  const togglePause = async () => {
    try {
      if (paused) {
        const resumed = await resume.mutateAsync();
        toast.show(t('tasks.resume.done', { date: resumed.nextDueOn ? formatDay(resumed.nextDueOn) : '' }));
      } else {
        await pause.mutateAsync();
        toast.show(t('tasks.pause.done'));
      }
    } catch {
      return;
    }
    router.back();
  };

  const deleteTask = async () => {
    const ok = await confirm({
      title: t('tasks.delete.confirmTitle', { name: taskTitle(task, t) }),
      message: t('tasks.delete.confirmMessage'),
      confirmLabel: t('tasks.delete.confirm'),
      cancelLabel: t('common.cancel'),
      destructive: true,
    });
    if (!ok) return;
    try {
      await remove.mutateAsync();
    } catch {
      return;
    }
    toast.show(t('tasks.delete.done'));
    router.back();
  };

  const actionError = pause.error ?? resume.error ?? remove.error;

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="interactive"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space['2xl'] }}
    >
      <View style={{ paddingHorizontal: theme.space.lg }}>
        <TaskForm
          initial={initial}
          defaults={defaults}
          creating={false}
          dueLabel={t('tasks.edit.nextDueOn')}
          dueHint={t('tasks.edit.dueHint')}
          minDueOn={editMinDueOn(toDayString(new Date()), initial.nextDueOn)}
          submitLabel={t('common.save')}
          pending={update.isPending}
          error={update.error}
          onSubmit={(value) => {
            const request = toUpdateTaskRequest(value, initial);
            update.mutate(request, {
              onSuccess: (saved) => {
                toast.show(t('common.saved'));
                router.back();
                // 予定日を変えたときだけ、サーバーが植え替えの時期を確かめ直す（16.3c）
                if (request.nextDueOn !== undefined) void alertRepottingAvoid(saved);
              },
            });
          }}
        />
      </View>

      <View>
        <ListRow
          title={t(paused ? 'tasks.resume.action' : 'tasks.pause.action')}
          subtitle={t(paused ? 'tasks.resume.hint' : 'tasks.pause.hint')}
          disabled={busy}
          onPress={() => void togglePause()}
        />
        <ListRow title={t('tasks.delete.action')} destructive disabled={busy} onPress={() => void deleteTask()} />
        {actionError && (
          <AppText variant="footnote" color="danger" style={{ padding: theme.space.lg }}>
            {t(errorMessageKey(actionError))}
          </AppText>
        )}
      </View>
    </ScrollView>
  );
}
