import { useState } from 'react';

import type { TodayItem } from './todayList';
import { useToday } from './useToday';

/**
 * シートで開いた、今日やることの1件。開いたときの内容を覚えておく。
 * 完了・スキップは今日の一覧から楽観的に外すので、一覧から探し直すと、通信の途中でシートの中身が消えるため
 */
export function useOpenedTodayItem(householdId: string, taskId: string) {
  const today = useToday(householdId);
  const found = today.data?.items.find((i) => i.task.id === taskId);
  const [opened, setOpened] = useState<TodayItem | undefined>(found);
  // 開いたときに一覧がまだ読み込み中だった場合は、読み込めたときの内容を覚える
  if (opened === undefined && found !== undefined) setOpened(found);
  return { item: opened, loading: !today.data };
}
