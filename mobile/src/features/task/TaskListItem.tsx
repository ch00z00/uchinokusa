import { useTranslation } from 'react-i18next';

import { Badge } from '@/components/Badge';
import { CareKindIcon } from '@/components/CareKindIcon';
import { ListRow } from '@/components/ListRow';
import { formatDay } from '@/lib/formatDate';

import { currentInterval, normalizeCareKind, type CareTask } from './careTask';
import { formatInterval, taskTitle } from './taskText';

type Props = {
  task: CareTask;
  /** 押すとタスクの編集（16.3b。編集者以上）。渡さなければ押せない */
  onPress?: () => void;
};

/**
 * 植物の詳細のタスク（foundations.md の 5章の TaskListItem。PLANT-10、要件8-2）。
 * 左に作業の種類のアイコン、題名に作業の名前、補足に周期（季節ごとの周期なら「今：夏 4日ごと」）・次の予定日・実行条件。
 * 一時停止中ならバッジを出す
 */
export function TaskListItem({ task, onPress }: Props) {
  const { t } = useTranslation();
  const { interval, season } = currentInterval(task, new Date());
  const intervalText = formatInterval(interval, t);
  const paused = task.status === 'paused';
  const summary = [
    season ? t('tasks.list.seasonalNow', { season: t(`care.season.${season}`), interval: intervalText }) : intervalText,
    // 一時停止中は予定・今日やることに出ないので、次の予定日を出さない
    !paused && task.nextDueOn ? t('tasks.list.next', { date: formatDay(task.nextDueOn) }) : undefined,
  ]
    .filter(Boolean)
    .join(t('tasks.list.separator'));
  return (
    <ListRow
      leading={<CareKindIcon kind={normalizeCareKind(task.kind)} size="lg" />}
      title={taskTitle(task, t)}
      accessory={paused ? <Badge label={t('tasks.list.paused')} /> : undefined}
      subtitle={task.conditionText ? `${summary}\n${task.conditionText}` : summary}
      navigates={onPress !== undefined}
      onPress={onPress}
    />
  );
}
