import { SegmentedControl } from '@expo/ui/community/segmented-control';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { headerRightTextButton } from '@/components/HeaderTextButton';
import { ListRow } from '@/components/ListRow';
import { useToast } from '@/components/Toast';
import { can } from '@/features/household/permissions';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import { useLocations } from '@/features/location/useLocations';
import type { Plant } from '@/features/plant/plantList';
import { usePlant, useUnarchivePlant } from '@/features/plant/usePlants';
import { confirm } from '@/lib/confirm';
import { TaskListItem } from '@/features/task/TaskListItem';
import { useCareTasks } from '@/features/task/useCareTasks';
import { formatDate, formatDayWithYear } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// 詳細の中のタブ（2026-11-22 に決定）。履歴・メモ・カレンダーはタスク 17.3 で作る
const sections = ['overview', 'history', 'notes', 'calendar'] as const;
type Section = (typeof sections)[number];

// PLANT-10 植物の詳細。上の OS のセグメントの切り替えで、概要・履歴・メモ・カレンダーを切り替える
export default function PlantDetailScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { state } = useSelectedHousehold();
  const plant = usePlant(id);
  const householdId = state.status === 'selected' ? state.household.id : '';
  const canEdit = state.status === 'selected' && can(state.household.role, 'edit');
  const [section, setSection] = useState<Section>('overview');

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.xl }}
    >
      <Stack.Screen
        options={{
          title: plant.data?.name ?? '',
          // 編集（PLANT-02）は編集者以上。アーカイブした植物も、削除のために開ける
          ...(canEdit && plant.data
            ? headerRightTextButton({
                title: t('plants.edit.open'),
                onPress: () => router.push({ pathname: '/plants/[id]/edit', params: { id } }),
              })
            : {}),
        }}
      />
      {plant.isError ? (
        <AppText color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
          {t(errorMessageKey(plant.error))}
        </AppText>
      ) : !plant.data ? (
        <ActivityIndicator color={theme.color.text.secondary} />
      ) : (
        <>
          <View style={{ paddingHorizontal: theme.space.lg }}>
            <SegmentedControl
              accessibilityLabel={t('plantDetail.sectionsLabel')}
              values={sections.map((s) => t(`plantDetail.sections.${s}`))}
              selectedIndex={sections.indexOf(section)}
              onChange={(e) => setSection(sections[e.nativeEvent.selectedSegmentIndex] ?? 'overview')}
            />
          </View>
          {section === 'overview' ? (
            <Overview householdId={householdId} plant={plant.data} canEdit={canEdit} />
          ) : (
            <AppText color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
              {t('plantDetail.comingSoon')}
            </AppText>
          )}
        </>
      )}
    </ScrollView>
  );
}

/** 概要：アーカイブ中の表示、植物の情報、お手入れタスクの一覧（要件8-2） */
function Overview({ householdId, plant, canEdit }: { householdId: string; plant: Plant; canEdit: boolean }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const locations = useLocations(householdId);
  const tasks = useCareTasks(plant.id);
  const archived = plant.archivedAt !== undefined;
  // アーカイブ中の植物には、タスクの操作（追加・編集）を出さない（PLANT-10）
  const canAddTask = canEdit && !archived;
  const locationName = plant.locationId ? locations.data?.find((l) => l.id === plant.locationId)?.name : undefined;
  const species = plant.speciesName ?? plant.speciesFreeText;

  return (
    <>
      {archived && (
        <View style={{ paddingHorizontal: theme.space.lg }}>
          <ArchivedNotice householdId={householdId} plant={plant} canEdit={canEdit} />
        </View>
      )}
      <View>
        <ListRow title={t('plantDetail.species')} detail={species ?? t('plantDetail.unset')} />
        <ListRow title={t('plantDetail.location')} detail={locationName ?? t('plants.list.locationUnset')} />
        <ListRow
          title={t('plantDetail.acquiredOn')}
          detail={plant.acquiredOn ? formatDayWithYear(plant.acquiredOn) : t('plantDetail.unset')}
        />
      </View>
      <View style={{ gap: theme.space.sm }}>
        <AppText variant="headline" style={{ paddingHorizontal: theme.space.lg }}>
          {t('tasks.list.title')}
        </AppText>
        {tasks.isError ? (
          <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.md }}>
            <AppText color="secondary">{t(errorMessageKey(tasks.error))}</AppText>
            <Button title={t('common.retry')} variant="secondary" onPress={() => void tasks.refetch()} />
          </View>
        ) : !tasks.data ? (
          <ActivityIndicator color={theme.color.text.secondary} />
        ) : (
          <View>
            {tasks.data.length === 0 && (
              <AppText color="secondary" style={{ paddingHorizontal: theme.space.lg, paddingBottom: theme.space.md }}>
                {t(canAddTask ? 'tasks.list.emptyEditor' : 'tasks.list.empty')}
              </AppText>
            )}
            {tasks.data.map((task) => (
              <TaskListItem
                key={task.id}
                task={task}
                // 編集（TASK-01）は編集者以上で、アーカイブ中でない植物だけ
                onPress={
                  canAddTask
                    ? () =>
                        router.push({
                          pathname: '/plants/[id]/tasks/[taskId]',
                          params: { id: plant.id, taskId: task.id },
                        })
                    : undefined
                }
              />
            ))}
            {canAddTask && (
              <ListRow
                title={t('tasks.list.add')}
                accent
                onPress={() => router.push({ pathname: '/plants/[id]/tasks/new', params: { id: plant.id } })}
              />
            )}
          </View>
        )}
      </View>
    </>
  );
}

/** アーカイブ中の植物の表示（PLANT-10）。編集者以上には、アーカイブの解除を出す */
function ArchivedNotice({ householdId, plant, canEdit }: { householdId: string; plant: Plant; canEdit: boolean }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  const unarchive = useUnarchivePlant(householdId, plant.id);

  const restore = async () => {
    const ok = await confirm({
      title: t('plants.unarchive.confirmTitle', { name: plant.name }),
      message: t('plants.unarchive.confirmMessage'),
      confirmLabel: t('plants.unarchive.confirm'),
      cancelLabel: t('common.cancel'),
    });
    if (!ok) return;
    // 成功すると植物がアーカイブ中でなくなり、この表示ごと消える。消えた部品には mutate の onSuccess が
    // 呼ばれないので、mutateAsync で完了を待ってから知らせる。失敗は unarchive.error で下に出す
    try {
      await unarchive.mutateAsync();
    } catch {
      return;
    }
    toast.show(t('plants.unarchive.done'));
    // 植物の一覧へ戻り、植物が一覧に戻ったことを見せる（2026-11-22）
    router.dismissTo('/plants');
  };

  return (
    <View
      style={{
        gap: theme.space.md,
        padding: theme.space.lg,
        borderRadius: theme.radius.md,
        backgroundColor: theme.color.bg.subtle,
      }}
    >
      <AppText variant="headline">{t('plants.archive.badge')}</AppText>
      <AppText color="secondary">
        {t('plants.archive.notice', { date: plant.archivedAt ? formatDate(plant.archivedAt) : '' })}
      </AppText>
      {canEdit && (
        <Button
          title={t('plants.unarchive.action')}
          variant="secondary"
          loading={unarchive.isPending}
          onPress={() => void restore()}
        />
      )}
      {unarchive.error && (
        <AppText variant="footnote" color="danger">
          {t(errorMessageKey(unarchive.error))}
        </AppText>
      )}
    </View>
  );
}
