import type { components } from '@/api/schema';

export type CareTask = components['schemas']['CareTask'];
export type CareKind = components['schemas']['CareKind'];
export type Interval = components['schemas']['Interval'];
export type IntervalUnit = components['schemas']['IntervalUnit'];
export type SeasonalIntervals = components['schemas']['SeasonalIntervals'];
export type CareTaskDefault = components['schemas']['CareTaskDefault'];

/** 作業の種類（要件8-1）。並びは、選ぶときと一覧の順 */
export const careKinds: readonly CareKind[] = ['watering', 'fertilizing', 'repotting', 'pruning', 'other'];
export const intervalUnits: readonly IntervalUnit[] = ['day', 'week', 'month'];

export type Season = keyof SeasonalIntervals;
export const seasons: readonly Season[] = ['spring', 'summer', 'autumn', 'winter'];

/** 知らない作業の種類は「その他」として扱う（API の CareKind の決まり。値は今後も増えうる） */
export function normalizeCareKind(kind: string): CareKind {
  return careKinds.includes(kind as CareKind) ? (kind as CareKind) : 'other';
}

/** 日付の季節（サーバーと同じく北半球：春 3〜5月、夏 6〜8月、秋 9〜11月、冬 12〜2月） */
export function seasonOf(date: Date): Season {
  const month = date.getMonth() + 1;
  if (month >= 3 && month <= 5) return 'spring';
  if (month >= 6 && month <= 8) return 'summer';
  if (month >= 9 && month <= 11) return 'autumn';
  return 'winter';
}

/**
 * 今の周期。今の季節の周期があればそれと季節を、なければ基本の周期を返す
 * （一覧の「今：夏 4日ごと」。設定のない季節は基本の周期を使う）
 */
export function currentInterval(
  task: Pick<CareTask, 'interval' | 'seasonalIntervals'>,
  now: Date,
): { interval: Interval; season: Season | undefined } {
  const season = seasonOf(now);
  const seasonal = task.seasonalIntervals[season];
  return seasonal ? { interval: seasonal, season } : { interval: task.interval, season: undefined };
}

const dayNumber = (date: string) => {
  const [y, m, d] = date.split('-').map(Number);
  return Date.UTC(y ?? 0, (m ?? 1) - 1, d ?? 1) / 86_400_000;
};

/**
 * タスクの次の予定日を過ぎた日数（要件9-3。植物の詳細で期限切れを示す、17.4）。0 なら期限切れではない。
 * 今日やること（TODAY-01）に出るタスクと合わせ、一時停止中や予定日がないタスクは 0 にする
 */
export function overdueDays(task: Pick<CareTask, 'status' | 'nextDueOn'>, today: string): number {
  if (task.status !== 'active' || !task.nextDueOn) return 0;
  return Math.max(0, dayNumber(today) - dayNumber(task.nextDueOn));
}
