import { useTranslation } from 'react-i18next';
import { LayoutAnimation } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { useSnackbar } from '@/components/Snackbar';
import { useToast } from '@/components/Toast';
import { taskTitle } from '@/features/task/taskText';
import { confirm } from '@/lib/confirm';
import { formatDay, formatTime, toDayString } from '@/lib/formatDate';

import { classifyCompletionError } from './completion';
import { toSkipRequest, type SkipForm } from './skip';
import type { TodayItem } from './todayList';
import { summarizeBatch } from './batch';
import { useCompleteTask, useCompleteTasks, useSkipTask, useUndoRecord } from './useCompletion';

/**
 * 完了・スキップの流れ（TODAY-02〜05、要件10-1〜10-8・11-2・20-5）。今日の一覧と、スキップ・実施日のシートで共通に使う。
 * 押した直後に一覧から外し（楽観的更新）、成功したら取り消しのスナックバーを5秒出す。
 * 失敗したら一覧を戻し、すでに完了されていれば誰がいつ記録したかを示して追加で記録するかを選ばせ、
 * 通信できなければもう一度行うよう促す。
 * どちらも、記録できた（またはユーザーが答えて終わった）なら true を返す（シートを閉じてよい）
 */
export function useCompletionFlow(householdId: string) {
  const { t } = useTranslation();
  const toast = useToast();
  const snackbar = useSnackbar();
  const completeTask = useCompleteTask(householdId);
  const skipTask = useSkipTask(householdId);
  const completeTasks = useCompleteTasks(householdId);
  const undoRecord = useUndoRecord(householdId);

  const undo = async (recordId: string) => {
    try {
      await undoRecord.mutateAsync(recordId);
      toast.show(t('today.complete.undone'));
    } catch (error) {
      toast.show(t(errorMessageKey(error)));
    }
  };

  /** 失敗を知らせる。すでに完了されていれば、追加で記録するかを聞いて、選ばれたら記録し直す */
  const handleFailure = async (error: unknown, item: TodayItem, retryExtra: () => Promise<boolean>) => {
    const failure = classifyCompletionError(error);
    if (failure.type === 'already_completed') {
      const { existing } = failure;
      // 今日の記録なら時刻だけ、ほかの日なら日付もつける
      const time =
        existing.performedOn === toDayString(new Date())
          ? formatTime(existing.performedAt)
          : `${formatDay(existing.performedOn)} ${formatTime(existing.performedAt)}`;
      const extra = await confirm({
        title: t('today.conflict.title'),
        message: t('today.conflict.message', { name: existing.actorDisplayName, time, task: taskTitle(item.task, t) }),
        // 初期の選択は「記録しない」（OS のアラートで、取り消しの側が強調される）
        cancelLabel: t('today.conflict.skip'),
        confirmLabel: t('today.conflict.extra'),
      });
      return extra ? retryExtra() : true;
    }
    if (failure.type === 'network') toast.show(t('today.complete.offline'));
    else if (failure.type === 'stale') toast.show(t('today.complete.stale'));
    else toast.show(t(errorMessageKey(failure.error)));
    return failure.type === 'stale';
  };

  const complete = async (
    item: TodayItem,
    options: { performedOn?: string; allowExtra?: boolean } = {},
  ): Promise<boolean> => {
    // 外れていく項目を、ぱっと消さずに詰める
    LayoutAnimation.configureNext(LayoutAnimation.Presets.easeInEaseOut);
    try {
      const result = await completeTask.mutateAsync({
        taskId: item.task.id,
        cycleSeq: item.task.cycleSeq,
        ...options,
      });
      snackbar.show({
        message: options.allowExtra
          ? t('today.conflict.extraDone')
          : t('today.complete.done', { plant: item.plant.name, task: taskTitle(item.task, t) }),
        actionLabel: t('today.complete.undo'),
        onAction: () => void undo(result.record.id),
      });
      return true;
    } catch (error) {
      return handleFailure(error, item, () => complete(item, { ...options, allowExtra: true }));
    }
  };

  const skip = async (item: TodayItem, form: SkipForm): Promise<boolean> => {
    LayoutAnimation.configureNext(LayoutAnimation.Presets.easeInEaseOut);
    try {
      const result = await skipTask.mutateAsync({
        taskId: item.task.id,
        body: toSkipRequest(form, item.task.cycleSeq),
      });
      snackbar.show({
        message: t('today.skip.done', { date: formatDay(form.recheckOn) }),
        actionLabel: t('today.complete.undo'),
        onAction: () => void undo(result.record.id),
      });
      return true;
    } catch (error) {
      // スキップには「すでに完了」はないので、追加で記録し直すことはない
      return handleFailure(error, item, () => Promise.resolve(true));
    }
  };

  /**
   * まとめて完了（TODAY-06）。記録できなかったタスクの ID を返す（選んだまま残すため）。
   * 通信できないなど、まとめて失敗したときは、選んだすべてを返す
   */
  const completeMany = async (items: readonly TodayItem[]): Promise<string[]> => {
    LayoutAnimation.configureNext(LayoutAnimation.Presets.easeInEaseOut);
    try {
      const { results } = await completeTasks.mutateAsync(
        items.map((i) => ({ taskId: i.task.id, cycleSeq: i.task.cycleSeq })),
      );
      const { recordIds, alreadyCount, failedTaskIds } = summarizeBatch(results);
      if (recordIds.length > 0) {
        const done = t('today.batch.done', { count: recordIds.length });
        snackbar.show({
          message: alreadyCount > 0 ? `${done}${t('today.batch.already', { count: alreadyCount })}` : done,
          actionLabel: t('today.complete.undo'),
          // まとめて記録した分を、まとめて取り消す
          onAction: () => void undoMany(recordIds),
        });
      } else if (alreadyCount > 0) {
        toast.show(t('today.batch.allAlready', { count: alreadyCount }));
      }
      if (failedTaskIds.length > 0) toast.show(t('today.batch.failed', { count: failedTaskIds.length }));
      return failedTaskIds;
    } catch (error) {
      const failure = classifyCompletionError(error);
      toast.show(failure.type === 'network' ? t('today.complete.offline') : t(errorMessageKey(error)));
      return items.map((i) => i.task.id);
    }
  };

  const undoMany = async (recordIds: readonly string[]) => {
    const results = await Promise.allSettled(recordIds.map((id) => undoRecord.mutateAsync(id)));
    const failed = results.find((r) => r.status === 'rejected');
    toast.show(failed ? t(errorMessageKey(failed.reason)) : t('today.complete.undone'));
  };

  return {
    complete,
    skip,
    completeMany,
    pending: completeTask.isPending || skipTask.isPending,
    batchPending: completeTasks.isPending,
  };
}
