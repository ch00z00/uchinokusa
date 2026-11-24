import type { TodayItem, TodayList } from '@/features/today/todayList';

/** 今日やることの取得の結果（日付ごと）から、タスクの項目を探す */
export function findTodayItem(lists: readonly (TodayList | undefined)[], taskId: string): TodayItem | undefined {
  for (const list of lists) {
    const item = list?.items.find((i) => i.task.id === taskId);
    if (item) return item;
  }
  return undefined;
}
