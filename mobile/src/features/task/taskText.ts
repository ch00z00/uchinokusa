import type { TFunction } from 'i18next';

import { normalizeCareKind, type CareTask, type Interval } from './careTask';

/** 周期の文（「7日ごと」「2週ごと」「24か月ごと」） */
export function formatInterval(interval: Interval, t: TFunction): string {
  return t(`care.interval.${interval.unit}`, { count: interval.value });
}

/** タスクの名前。「その他」は作業名（なければ「その他」）、ほかは作業の種類の名前 */
export function taskTitle(task: Pick<CareTask, 'kind' | 'customName'>, t: TFunction): string {
  const kind = normalizeCareKind(task.kind);
  if (kind === 'other' && task.customName) return task.customName;
  return t(`care.kind.${kind}`);
}
