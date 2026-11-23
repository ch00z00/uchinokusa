import { MenuView } from '@expo/ui/community/menu';
import type { TFunction } from 'i18next';
import { router, Stack } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, RefreshControl, SectionList, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { EmptyState } from '@/components/EmptyState';
import { TaskCard } from '@/components/TaskCard';
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
import { useToday, useTodayGrouping } from '@/features/today/useToday';
import { formatDay } from '@/lib/formatDate';
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
      renderItem={({ item }) => <TodayTaskCard item={item} />}
      ListEmptyComponent={<TodayEmpty householdId={householdId} next={list.next} canEdit={canEdit} />}
    />
  );
}

/** 今日やることの1件。押すと植物の詳細（PLANT-10）。完了などの操作は 17.2 で足す */
function TodayTaskCard({ item }: { item: TodayItem }) {
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
    />
  );
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
function groupingMenuOptions(
  grouping: TodayGrouping,
  setGrouping: (next: TodayGrouping) => void,
  t: TFunction,
) {
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
