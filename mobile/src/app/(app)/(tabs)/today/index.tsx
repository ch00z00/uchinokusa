import { MenuView } from '@expo/ui/community/menu';
import type { TFunction } from 'i18next';
import { Link, router, Stack } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { ActivityIndicator, Alert, Platform, Pressable, RefreshControl, SectionList, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { CompleteButton } from '@/components/CompleteButton';
import { EmptyState } from '@/components/EmptyState';
import { HeaderTextButton } from '@/components/HeaderTextButton';
import { SelectionCheck } from '@/components/SelectionCheck';
import { TaskCard } from '@/components/TaskCard';
import { can } from '@/features/household/permissions';
import { householdSwitcherOptions } from '@/features/household/HouseholdSwitcher';
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
import { toggleSelection } from '@/features/today/batch';
import { useCompletionFlow } from '@/features/today/useCompletionFlow';
import { useLongPressHint, useToday, useTodayGrouping } from '@/features/today/useToday';
import { formatDay } from '@/lib/formatDate';
import { useTheme } from '@/theme';
import { space } from '@/theme/tokens';

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
  // 引っぱって取り直しているあいだだけ、スピナーを出す。シートを開いたときや完了のあとの裏の取り直し
  // （isRefetching）で出すと、一覧が下にずれてスピナーが出たままに見えるため（2026-11-23）
  const [pulling, setPulling] = useState(false);
  // まとめて完了の選択モード（TODAY-06）。null なら選択モードでない
  const [selection, setSelection] = useState<Set<string> | null>(null);
  const items = today.data?.items ?? [];
  // 今日のタスクがなくなったら、選択モードを終える
  if (selection !== null && today.data && items.length === 0) setSelection(null);
  // 一覧にないタスク（ほかの人が記録した、など）は、選んだものから外して数える
  const selectedIds = selection === null ? [] : items.filter((i) => selection.has(i.task.id)).map((i) => i.task.id);
  const pullToRefresh = async () => {
    setPulling(true);
    try {
      await today.refetch();
    } finally {
      setPulling(false);
    }
  };

  return (
    <>
      <Stack.Screen
        options={
          selection !== null
            ? selectionHeaderOptions({
                allSelected: selectedIds.length === items.length,
                onToggleAll: () =>
                  setSelection(selectedIds.length === items.length ? new Set() : new Set(items.map((i) => i.task.id))),
                onCancel: () => setSelection(null),
                t,
              })
            : {
                ...householdSwitcherOptions,
                ...groupingMenuOptions(grouping, setGrouping, t, {
                  // 選択（まとめて完了）は、編集者以上で、今日のタスクがあるときだけ
                  onSelect: canEdit && items.length > 0 ? () => setSelection(new Set()) : undefined,
                }),
              }
        }
      />
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
          refreshing={pulling}
          onRefresh={() => void pullToRefresh()}
          selection={selection}
          selectedIds={selectedIds}
          onSelectionChange={setSelection}
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
  selection,
  selectedIds,
  onSelectionChange,
}: {
  householdId: string;
  list: TodayList;
  grouping: TodayGrouping;
  canEdit: boolean;
  refreshing: boolean;
  onRefresh: () => void;
  /** 選択モードで選んだタスク。null なら選択モードでない */
  selection: Set<string> | null;
  /** 選んだタスクのうち、一覧にあるもの */
  selectedIds: string[];
  onSelectionChange: (next: Set<string> | null) => void;
}) {
  const { t } = useTranslation();
  const theme = useTheme();
  const insets = useSafeAreaInsets();
  const { complete, completeMany, batchPending } = useCompletionFlow(householdId);
  const selecting = selection !== null;
  // 下の「N件を完了」の位置（Toast・Snackbar と同じく、タブバーの上）
  const barBottom = insets.bottom + theme.space['2xl'] * 2;

  const submitBatch = async () => {
    const chosen = list.items.filter((i) => selectedIds.includes(i.task.id));
    const failed = await completeMany(chosen);
    // 全部うまくいけば選択モードを終え、失敗した分は選んだまま残す
    onSelectionChange(failed.length === 0 ? null : new Set(failed));
  };
  const [showHint, markHintUsed] = useLongPressHint();
  const sections = groupTodayItems(list.items, grouping, t('plants.list.locationUnset')).map((section) => ({
    ...section,
    // 作業の種類ごとの見出しは、種類の名前
    title: grouping === 'kind' ? t(`care.kind.${section.key as CareKind}`) : section.title,
    data: section.items,
  }));

  return (
    <>
      <SectionList
        contentInsetAdjustmentBehavior="automatic"
        sections={list.items.length === 0 ? [] : sections}
        keyExtractor={(item) => item.task.id}
        // 選択モードでは、下の「N件を完了」に最後の項目が隠れないように、下に余白を足す
        contentContainerStyle={{
          padding: theme.space.lg,
          gap: theme.space.md,
          paddingBottom: selecting ? barBottom + theme.size.tapMin + theme.space.lg : theme.space.lg,
        }}
        stickySectionHeadersEnabled={false}
        refreshControl={<RefreshControl refreshing={refreshing} onRefresh={onRefresh} />}
        renderSectionHeader={({ section }) =>
          section.title ? (
            <AppText variant="headline" style={{ paddingTop: theme.space.md }}>
              {section.title}
            </AppText>
          ) : null
        }
        ListHeaderComponent={
          // 長押しのメニューは気づきにくいので、一度使うまで知らせる（編集者以上）
          canEdit && showHint && !selecting && list.items.length > 0 ? (
            <AppText variant="footnote" color="secondary">
              {t('today.longPressHint')}
            </AppText>
          ) : undefined
        }
        renderItem={({ item }) =>
          selecting ? (
            <TodayTaskCard
              item={item}
              selected={selection.has(item.task.id)}
              onToggle={() => onSelectionChange(toggleSelection(selection, item.task.id))}
            />
          ) : (
            <TodayTaskCard
              item={item}
              onComplete={canEdit ? () => void complete(item) : undefined}
              onMenuUsed={canEdit ? markHintUsed : undefined}
            />
          )
        }
        ListEmptyComponent={<TodayEmpty householdId={householdId} next={list.next} canEdit={canEdit} />}
      />
      {selecting && (
        <View
          pointerEvents="box-none"
          style={{ position: 'absolute', left: 0, right: 0, bottom: barBottom, paddingHorizontal: theme.space.lg }}
        >
          <Button
            title={t('today.select.submit', { count: selectedIds.length })}
            disabled={selectedIds.length === 0}
            loading={batchPending}
            onPress={() => void submitBatch()}
          />
        </View>
      )}
    </>
  );
}

/**
 * 今日やることの1件。押すと植物の詳細（PLANT-10）。編集者以上には、右に完了のボタンと、長押しのメニュー
 * （スキップ・実施日を変えて記録・植物の詳細。iOS はリンクのコンテキストメニュー、Android は OS のアラートの選択肢）
 */
function TodayTaskCard({
  item,
  onComplete,
  onMenuUsed,
  selected,
  onToggle,
}: {
  item: TodayItem;
  onComplete?: () => void;
  /** 選択モードで選んでいるか。渡すと、押すと選ぶ・外す（onToggle）カードになる */
  selected?: boolean;
  onToggle?: () => void;
  /** 長押しのメニューを使ったとき（知らせを消す）。閲覧者には渡さない（メニューを出さない） */
  onMenuUsed?: () => void;
}) {
  const { t } = useTranslation();
  const suggestion = todaySuggestion(item);
  const due = item.overdueDays === 0 || !item.task.nextDueOn ? t('today.dueToday') : formatDay(item.task.nextDueOn);
  const meta = [taskTitle(item.task, t), item.plant.locationName, due]
    .filter((part): part is string => Boolean(part))
    .join(t('today.separator'));
  const plantHref = { pathname: '/plants/[id]' as const, params: { id: item.plant.id } };
  const openSheet = (sheet: 'skip' | 'record') => {
    onMenuUsed?.();
    router.push({ pathname: `/today/[taskId]/${sheet}`, params: { taskId: item.task.id } });
  };
  // Android：長押しで OS のアラートの選択肢を出す
  const showAndroidMenu = () =>
    Alert.alert(taskTitle(item.task, t), item.plant.name, [
      { text: t('today.menu.skip'), onPress: () => openSheet('skip') },
      { text: t('today.menu.record'), onPress: () => openSheet('record') },
      { text: t('common.cancel'), style: 'cancel' },
    ]);

  if (onToggle) {
    return (
      <TaskCard
        kind={normalizeCareKind(item.task.kind)}
        title={item.plant.name}
        meta={meta}
        overdueDays={item.overdueDays}
        conditionText={item.task.conditionText}
        selected={selected}
        onPress={onToggle}
        trailing={<SelectionCheck selected={selected === true} />}
      />
    );
  }
  const card = (
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
      onPress={() => router.push(plantHref)}
      onLongPress={onMenuUsed && Platform.OS !== 'ios' ? showAndroidMenu : undefined}
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
  if (!onMenuUsed || Platform.OS !== 'ios') return card;
  return (
    <Link href={plantHref} asChild>
      <Link.Trigger>{card}</Link.Trigger>
      <Link.Menu>
        <Link.MenuAction title={t('today.menu.skip')} icon="forward.end" onPress={() => openSheet('skip')} />
        <Link.MenuAction title={t('today.menu.record')} icon="calendar" onPress={() => openSheet('record')} />
        <Link.MenuAction
          title={t('today.menu.open')}
          icon="leaf"
          onPress={() => {
            onMenuUsed();
            router.push(plantHref);
          }}
        />
      </Link.Menu>
    </Link>
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
  { onSelect }: { onSelect?: () => void },
) {
  const label = t('today.grouping.label');
  const selectButton = onSelect ? <HeaderTextButton title={t('today.select.start')} onPress={onSelect} /> : null;
  return {
    // iOS では、右端に「選択」、その左に ↑↓ のメニューが並ぶ（iOS の写真と同じく「選択」が右端）
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
      ...(selectButton ? [{ type: 'custom' as const, element: selectButton, hidesSharedBackground: true }] : []),
    ],
    headerRight: () => (
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: space.lg }}>
        {selectButton}
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
      </View>
    ),
  };
}

/** 選択モードのヘッダー（TODAY-06）。左に「すべて選択」（全部選んでいれば「選択を解除」）、右に「キャンセル」 */
function selectionHeaderOptions({
  allSelected,
  onToggleAll,
  onCancel,
  t,
}: {
  allSelected: boolean;
  onToggleAll: () => void;
  onCancel: () => void;
  t: TFunction;
}) {
  const left = (
    <HeaderTextButton title={t(allSelected ? 'today.select.none' : 'today.select.all')} onPress={onToggleAll} />
  );
  const right = <HeaderTextButton title={t('today.select.cancel')} onPress={onCancel} />;
  return {
    headerLeft: () => left,
    unstable_headerLeftItems: () => [{ type: 'custom' as const, element: left, hidesSharedBackground: true }],
    headerRight: () => right,
    unstable_headerRightItems: () => [{ type: 'custom' as const, element: right, hidesSharedBackground: true }],
  };
}
