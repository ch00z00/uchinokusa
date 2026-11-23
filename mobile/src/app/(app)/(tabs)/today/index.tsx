import { MenuView } from '@expo/ui/community/menu';
import type { TFunction } from 'i18next';
import { router, Stack } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, LayoutAnimation, Pressable, RefreshControl, SectionList, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { CompleteButton } from '@/components/CompleteButton';
import { EmptyState } from '@/components/EmptyState';
import { useSnackbar } from '@/components/Snackbar';
import { TaskCard } from '@/components/TaskCard';
import { useToast } from '@/components/Toast';
import { can } from '@/features/household/permissions';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { usePlants } from '@/features/plant/usePlants';
import { todaySuggestionText } from '@/features/suggestion/suggestionText';
import { normalizeCareKind, type CareKind } from '@/features/task/careTask';
import { taskTitle } from '@/features/task/taskText';
import {
  groupTodayItems,
  todayGroupings,
  todaySuggestion,
  type TodayGrouping,
  type TodayItem,
  type TodayList,
} from '@/features/today/todayList';
import { classifyCompletionError } from '@/features/today/completion';
import { useCompleteTask, useUndoRecord } from '@/features/today/useCompletion';
import { useToday, useTodayGrouping } from '@/features/today/useToday';
import { confirm } from '@/lib/confirm';
import { formatDay, formatTime, toDayString } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// TODAY-01 今日やること（ホーム。要件9）。予定日が今日以前の未完了のタスクを、TaskCard で並べる。
// 表示の切り替え（予定日順・置き場所ごと・作業の種類ごと）は、ヘッダーの右の OS のメニュー（2026-11-23 に決定）
export default function TodayScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { state } = useSelectedHousehold();
  const householdId = state.status === 'selected' ? state.household.id : '';
  const canEdit = state.status === 'selected' && can(state.household.role, 'edit');
  const today = useToday(householdId);
  const [grouping, setGrouping] = useTodayGrouping();

  return (
    <>
      <Stack.Screen options={groupingMenuOptions(grouping, setGrouping, t)} />
      {today.isError && !today.data ? (
        <View style={{ padding: theme.space.lg, gap: theme.space.md }}>
          <AppText color="secondary">{t(errorMessageKey(today.error))}</AppText>
          <Button title={t('common.retry')} variant="secondary" onPress={() => void today.refetch()} />
        </View>
      ) : !today.data ? (
        <View style={{ padding: theme.space.xl }}>
          <ActivityIndicator color={theme.color.text.secondary} />
        </View>
      ) : (
        <TodaySections
          householdId={householdId}
          list={today.data}
          grouping={grouping}
          canEdit={canEdit}
          refreshing={today.isRefetching}
          onRefresh={() => void today.refetch()}
        />
      )}
    </>
  );
}

function TodaySections({
  householdId,
  list,
  grouping,
  canEdit,
  refreshing,
  onRefresh,
}: {
  householdId: string;
  list: TodayList;
  grouping: TodayGrouping;
  canEdit: boolean;
  refreshing: boolean;
  onRefresh: () => void;
}) {
  const { t } = useTranslation();
  const theme = useTheme();
  const complete = useCompletionFlow(householdId);
  const sections = groupTodayItems(list.items, grouping, t('plants.list.locationUnset')).map((section) => ({
    ...section,
    // 作業の種類ごとの見出しは、種類の名前
    title: grouping === 'kind' ? t(`care.kind.${section.key as CareKind}`) : section.title,
    data: section.items,
  }));

  return (
    <SectionList
      contentInsetAdjustmentBehavior="automatic"
      sections={list.items.length === 0 ? [] : sections}
      keyExtractor={(item) => item.task.id}
      contentContainerStyle={{ padding: theme.space.lg, gap: theme.space.md }}
      stickySectionHeadersEnabled={false}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={onRefresh} />}
      renderSectionHeader={({ section }) =>
        section.title ? (
          <AppText variant="headline" style={{ paddingTop: theme.space.md }}>
            {section.title}
          </AppText>
        ) : null
      }
      renderItem={({ item }) => (
        <TodayTaskCard item={item} onComplete={canEdit ? () => void complete(item) : undefined} />
      )}
      ListEmptyComponent={<TodayEmpty householdId={householdId} next={list.next} canEdit={canEdit} />}
    />
  );
}

/** 今日やることの1件。押すと植物の詳細（PLANT-10）。編集者以上には、右に完了のボタン（スキップなどは 17.2b） */
function TodayTaskCard({ item, onComplete }: { item: TodayItem; onComplete?: () => void }) {
  const { t } = useTranslation();
  const suggestion = todaySuggestion(item);
  const due = item.overdueDays === 0 || !item.task.nextDueOn ? t('today.dueToday') : formatDay(item.task.nextDueOn);
  const meta = [taskTitle(item.task, t), item.plant.locationName, due]
    .filter((part): part is string => Boolean(part))
    .join(t('today.separator'));
  return (
    <TaskCard
      kind={normalizeCareKind(item.task.kind)}
      title={item.plant.name}
      meta={meta}
      overdueDays={item.overdueDays}
      conditionText={item.task.conditionText}
      suggestion={
        suggestion && (suggestion.kind === 'avoid' || suggestion.kind === 'conditional')
          ? { kind: suggestion.kind, text: todaySuggestionText(suggestion, t) }
          : undefined
      }
      accessibilityHint={t('today.openPlantHint')}
      onPress={() => router.push({ pathname: '/plants/[id]', params: { id: item.plant.id } })}
      trailing={
        onComplete && (
          <CompleteButton
            accessibilityLabel={t('today.complete.label', { task: taskTitle(item.task, t) })}
            onPress={onComplete}
          />
        )
      }
    />
  );
}

/**
 * 完了の流れ（TODAY-02・TODAY-05、要件10-1・10-4・11-2・20-5）。
 * 押した直後に一覧から外し（useCompleteTask の楽観的更新）、成功したら取り消しのスナックバーを5秒出す。
 * 失敗したら一覧を戻し、すでに完了されていれば誰がいつ記録したかを示して追加で記録するかを選ばせ、
 * 通信できなければもう一度完了するよう促す
 */
function useCompletionFlow(householdId: string) {
  const { t } = useTranslation();
  const toast = useToast();
  const snackbar = useSnackbar();
  const completeTask = useCompleteTask(householdId);
  const undoRecord = useUndoRecord(householdId);

  const undo = async (recordId: string) => {
    try {
      await undoRecord.mutateAsync(recordId);
      toast.show(t('today.complete.undone'));
    } catch (error) {
      toast.show(t(errorMessageKey(error)));
    }
  };

  const complete = async (item: TodayItem, allowExtra = false): Promise<void> => {
    const task = taskTitle(item.task, t);
    // 外れていく項目を、ぱっと消さずに詰める
    LayoutAnimation.configureNext(LayoutAnimation.Presets.easeInEaseOut);
    try {
      const result = await completeTask.mutateAsync({ taskId: item.task.id, cycleSeq: item.task.cycleSeq, allowExtra });
      snackbar.show({
        message: allowExtra
          ? t('today.conflict.extraDone')
          : t('today.complete.done', { plant: item.plant.name, task }),
        actionLabel: t('today.complete.undo'),
        onAction: () => void undo(result.record.id),
      });
    } catch (error) {
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
          message: t('today.conflict.message', { name: existing.actorDisplayName, time, task }),
          // 初期の選択は「記録しない」（OS のアラートで、取り消しの側が強調される）
          cancelLabel: t('today.conflict.skip'),
          confirmLabel: t('today.conflict.extra'),
        });
        if (extra) await complete(item, true);
        return;
      }
      if (failure.type === 'network') toast.show(t('today.complete.offline'));
      else if (failure.type === 'stale') toast.show(t('today.complete.stale'));
      else toast.show(t(errorMessageKey(failure.error)));
    }
  };

  return complete;
}

/** 今日やることがないとき（要件9-6）。植物がなければ、植物の登録へ誘う */
function TodayEmpty({
  householdId,
  next,
  canEdit,
}: {
  householdId: string;
  next: TodayItem | undefined;
  canEdit: boolean;
}) {
  const { t } = useTranslation();
  // 植物の一覧（植物のタブと同じ取得。キャッシュがあれば通信しない）で、植物が1つもないかを見る
  const plants = usePlants(householdId, '');
  if (!next && plants.data?.length === 0) {
    return (
      <EmptyState
        title={t('today.noPlants.title')}
        message={t(canEdit ? 'today.noPlants.message' : 'today.noPlants.viewerMessage')}
      >
        {canEdit && <Button title={t('today.noPlants.action')} onPress={() => router.push('/plants/new')} />}
      </EmptyState>
    );
  }
  return (
    <EmptyState
      title={t('today.empty.title')}
      message={
        next?.task.nextDueOn
          ? t('today.empty.next', {
              plant: next.plant.name,
              task: taskTitle(next.task, t),
              date: formatDay(next.task.nextDueOn),
            })
          : t('today.empty.noSchedule')
      }
    />
  );
}

/**
 * ヘッダーの右の、表示の切り替えのメニュー（OS の層）。iOS はヘッダーのネイティブのメニューで、選んだ表示にチェックを付ける。
 * Android は @expo/ui の MenuView
 */
function groupingMenuOptions(grouping: TodayGrouping, setGrouping: (next: TodayGrouping) => void, t: TFunction) {
  const label = t('today.grouping.label');
  return {
    unstable_headerRightItems: () => [
      {
        type: 'menu' as const,
        label,
        icon: { type: 'sfSymbol' as const, name: 'arrow.up.arrow.down' as const },
        menu: {
          title: label,
          items: todayGroupings.map((g) => ({
            type: 'action' as const,
            label: t(`today.grouping.${g}`),
            state: g === grouping ? ('on' as const) : ('off' as const),
            onPress: () => setGrouping(g),
          })),
        },
      },
    ],
    headerRight: () => (
      <MenuView
        title={label}
        actions={todayGroupings.map((g) => ({
          id: g,
          title: t(`today.grouping.${g}`),
          state: g === grouping ? ('on' as const) : ('off' as const),
        }))}
        onPressAction={({ nativeEvent }) => setGrouping(nativeEvent.event as TodayGrouping)}
      >
        <Pressable accessibilityRole="button" accessibilityLabel={label}>
          <AppText color="accent">{label}</AppText>
        </Pressable>
      </MenuView>
    ),
  };
}
