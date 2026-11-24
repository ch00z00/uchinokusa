import { useQueryClient, type InfiniteData } from '@tanstack/react-query';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { DateField } from '@/components/DateField';
import { ListRow } from '@/components/ListRow';
import { useToast } from '@/components/Toast';
import type { PlantCalendar } from '@/features/calendar/calendarDays';
import { calendarKeys } from '@/features/calendar/useCalendar';
import { useSelectedHousehold } from '@/features/household/SelectedHouseholdProvider';
import {
  actorOf,
  findRecord,
  validateRecordDate,
  type CareRecord,
  type CareRecordPage,
} from '@/features/record/careRecord';
import { recordKeys, useDeleteRecord, useUpdateRecord } from '@/features/record/useRecords';
import { taskTitle } from '@/features/task/taskText';
import { confirm } from '@/lib/confirm';
import { toDayString } from '@/lib/formatDate';
import { useTheme } from '@/theme';

// PLANT-12 記録の編集・削除（OS のシート。要件12-4）。履歴（PLANT-11）の記録を押すと開く。編集者以上
export default function RecordEditScreen() {
  const { t } = useTranslation();
  const theme = useTheme();
  const { id, recordId } = useLocalSearchParams<{ id: string; recordId: string }>();
  const queryClient = useQueryClient();
  const { state } = useSelectedHousehold();
  const householdId = state.status === 'selected' ? state.household.id : '';
  // 記録1件を取る API はないので、履歴の取得の結果から探す。保存・削除で履歴を取り直しても、
  // 開いたときの記録を使い続ける（削除の途中で「見つかりません」に変わらないように）
  // 記録1件を取る API はないので、履歴とカレンダーの取得の結果から探す。保存・削除で取り直しても、
  // 開いたときの記録を使い続ける（削除の途中で「見つかりません」に変わらないように）
  const [record] = useState(() =>
    findRecord(
      [
        ...queryClient
          .getQueriesData<InfiniteData<CareRecordPage>>({ queryKey: recordKeys.all(id) })
          .map(([, data]) => data?.pages),
        ...queryClient
          .getQueriesData<PlantCalendar>({ queryKey: calendarKeys.all(id) })
          .map(([, data]) => (data ? [data] : undefined)),
      ],
      recordId,
    ),
  );
  if (!record) {
    return (
      <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={{ padding: theme.space.lg }}>
        <AppText color="secondary">{t('history.edit.missing')}</AppText>
      </ScrollView>
    );
  }
  return <RecordEditForm householdId={householdId} record={record} />;
}

function RecordEditForm({ householdId, record }: { householdId: string; record: CareRecord }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const toast = useToast();
  const update = useUpdateRecord(householdId, record.plantId, record.id);
  const remove = useDeleteRecord(householdId, record.plantId, record.id);
  const today = toDayString(new Date());
  const [performedOn, setPerformedOn] = useState(record.performedOn);
  const actor = actorOf(record);
  const dateError = validateRecordDate(performedOn, today);
  const busy = update.isPending || remove.isPending;

  // 保存・削除のあとは、シートを閉じて履歴へ戻る。成功すると履歴が取り直されるので、確実に戻れるように
  // mutateAsync で完了を待つ。失敗は各 error で下に出す
  const save = async () => {
    try {
      await update.mutateAsync(performedOn);
    } catch {
      return;
    }
    toast.show(t('common.saved'));
    router.back();
  };

  const deleteRecord = async () => {
    const ok = await confirm({
      title: t('history.delete.confirmTitle'),
      message: t('history.delete.confirmMessage'),
      confirmLabel: t('history.delete.confirm'),
      cancelLabel: t('common.cancel'),
      destructive: true,
    });
    if (!ok) return;
    try {
      await remove.mutateAsync();
    } catch {
      return;
    }
    toast.show(t('history.delete.done'));
    router.back();
  };

  const error = update.error ?? remove.error;

  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      contentContainerStyle={{ paddingVertical: theme.space.lg, gap: theme.space.xl }}
    >
      <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.lg }}>
        <AppText color="secondary">
          {t('history.edit.lead', {
            task: taskTitle(record, t),
            outcome: t(`history.outcome.${record.outcome}`),
            name: actor.status === 'member' ? actor.name : t('history.deletedUser'),
          })}
        </AppText>
        {/* 未来の日は選べない（API の決まり） */}
        <DateField
          label={t(record.outcome === 'skipped' ? 'history.edit.skippedOn' : 'history.edit.performedOn')}
          value={performedOn}
          onChange={setPerformedOn}
          maximumDate={today}
          clearable={false}
        />
        <AppText variant="footnote" color="secondary">
          {t('history.edit.hint')}
        </AppText>
        <Button
          title={t('common.save')}
          onPress={() => void save()}
          loading={update.isPending}
          disabled={busy || performedOn === record.performedOn || dateError !== undefined}
        />
      </View>
      <View>
        <ListRow title={t('history.delete.action')} destructive disabled={busy} onPress={() => void deleteRecord()} />
        {error && (
          <AppText variant="footnote" color="danger" style={{ padding: theme.space.lg }}>
            {t(errorMessageKey(error))}
          </AppText>
        )}
      </View>
    </ScrollView>
  );
}
