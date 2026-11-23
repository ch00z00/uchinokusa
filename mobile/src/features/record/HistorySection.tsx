import { router } from 'expo-router';
import { useEffect, useState, type MutableRefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, View } from 'react-native';

import { errorMessageKey } from '@/api/errorMessage';
import { AppText } from '@/components/AppText';
import { Button } from '@/components/Button';
import { FilterChips } from '@/components/FilterChips';
import { ListRow } from '@/components/ListRow';
import { taskTitle } from '@/features/task/taskText';
import { formatDay } from '@/lib/formatDate';
import { useTheme } from '@/theme';

import { flattenRecords, historyFilters, type HistoryFilter } from './careRecord';
import { RecordListItem } from './RecordListItem';
import { useRecords } from './useRecords';

type Props = {
  plantId: string;
  /** 記録の編集・削除（PLANT-12）ができるか。編集者以上で、アーカイブ中でない植物 */
  canEdit: boolean;
  /**
   * 詳細の画面のスクロールが下の端に近づいたときに呼ぶ関数を、ここに入れる（続きの読み込み）。
   * 履歴は詳細の画面のスクロールの中にあるので、自分ではスクロールの位置がわからない
   */
  endReachedRef: MutableRefObject<(() => void) | undefined>;
};

/**
 * 植物の詳細の「履歴」（PLANT-11、要件12-1〜12-3・12-5）。
 * 上から、作業の種類ごとの最終実施日、作業の種類の絞り込み、記録の一覧（新しい順、下へスクロールすると続きを読み込む）
 */
export function HistorySection({ plantId, canEdit, endReachedRef }: Props) {
  const { t } = useTranslation();
  const theme = useTheme();
  const [filter, setFilter] = useState<HistoryFilter>('all');
  const records = useRecords(plantId, filter);
  const list = flattenRecords(records.data?.pages);
  // 最終実施日は、絞り込みにかかわらずすべての種類が返る（API の決まり）
  const lastPerformed = records.data?.pages[0]?.lastPerformed ?? [];
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = records;

  useEffect(() => {
    endReachedRef.current = () => {
      if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
    };
    return () => {
      endReachedRef.current = undefined;
    };
  }, [endReachedRef, hasNextPage, isFetchingNextPage, fetchNextPage]);

  return (
    <>
      {lastPerformed.length > 0 && (
        <View style={{ gap: theme.space.sm }}>
          <AppText variant="headline" style={{ paddingHorizontal: theme.space.lg }}>
            {t('history.lastPerformed')}
          </AppText>
          <View>
            {lastPerformed.map((last) => (
              <ListRow
                key={`${last.kind}-${last.customName ?? ''}`}
                title={taskTitle(last, t)}
                detail={formatDay(last.performedOn)}
              />
            ))}
          </View>
        </View>
      )}
      <View style={{ gap: theme.space.sm }}>
        <AppText variant="headline" style={{ paddingHorizontal: theme.space.lg }}>
          {t('history.title')}
        </AppText>
        <FilterChips
          accessibilityLabel={t('history.filterLabel')}
          chips={historyFilters.map((key) => ({
            key,
            label: key === 'all' ? t('plants.list.filterAll') : t(`care.kind.${key}`),
          }))}
          selectedKey={filter}
          onSelect={(key) => setFilter(key as HistoryFilter)}
        />
        {records.isError ? (
          <View style={{ paddingHorizontal: theme.space.lg, gap: theme.space.md }}>
            <AppText color="secondary">{t(errorMessageKey(records.error))}</AppText>
            <Button title={t('common.retry')} variant="secondary" onPress={() => void records.refetch()} />
          </View>
        ) : records.isPending ? (
          <ActivityIndicator color={theme.color.text.secondary} />
        ) : list.length === 0 ? (
          <AppText color="secondary" style={{ paddingHorizontal: theme.space.lg }}>
            {t(filter === 'all' ? 'history.empty' : 'history.emptyFiltered')}
          </AppText>
        ) : (
          <View>
            {list.map((record) => (
              <RecordListItem
                key={record.id}
                record={record}
                onPress={
                  canEdit
                    ? () =>
                        router.push({
                          pathname: '/plants/[id]/records/[recordId]',
                          params: { id: plantId, recordId: record.id },
                        })
                    : undefined
                }
              />
            ))}
            {isFetchingNextPage && (
              <ActivityIndicator color={theme.color.text.secondary} style={{ padding: theme.space.lg }} />
            )}
          </View>
        )}
      </View>
    </>
  );
}
