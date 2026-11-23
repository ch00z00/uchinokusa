import type { TFunction } from 'i18next';

import type { components } from '@/api/schema';
import { formatDay } from '@/lib/formatDate';

export type Suggestion = components['schemas']['Suggestion'];

// 植え替えで避ける時期（サーバーの domain.RepottingPeriod のうち、避けるもの）。知らない値は理由に入れない
const avoidPeriods = ['midsummer', 'midwinter', 'avoid'] as const;
type AvoidPeriod = (typeof avoidPeriods)[number];

/**
 * 植え替えの提案の理由（要件17-7）。サーバーは文言ではなく材料を返す（docs/decisions/0028）：
 * afterAcclimation（慣らし期間を待ったか）、acclimationDays、avoided（飛ばした避ける時期）、species（種類ごとのルールを使った種類）
 */
export function repottingReason(params: Record<string, unknown>, t: TFunction): string {
  const parts: string[] = [];
  if (params.afterAcclimation === true && typeof params.acclimationDays === 'number') {
    parts.push(t('suggestion.repotting.reason.afterAcclimation', { days: params.acclimationDays }));
  }
  const avoided = Array.isArray(params.avoided)
    ? params.avoided.filter((p): p is AvoidPeriod => avoidPeriods.includes(p as AvoidPeriod))
    : [];
  if (avoided.length > 0) {
    const periods = avoided.map((p) => t(`suggestion.repotting.period.${p}`)).join(t('suggestion.repotting.and'));
    parts.push(t('suggestion.repotting.reason.avoided', { periods }));
  }
  parts.push(
    typeof params.species === 'string' && params.species !== ''
      ? t('suggestion.repotting.reason.species', { species: params.species })
      : t('suggestion.repotting.reason.general'),
  );
  return parts.join('');
}

/** 植物の登録のあとに出す、最初の植え替えの提案（未回答のもの）。見送った提案は、同じ植物に二度と出ない */
export function findInitialRepotting(suggestions: readonly Suggestion[], plantId: string): Suggestion | undefined {
  return suggestions.find(
    (s) => s.plantId === plantId && s.messageKey === 'repotting.initial' && s.status === 'pending',
  );
}

/**
 * そのタスクへの、植え替えの時期の注意（未回答のもの。要件17-8）。植え替えの予定日が避ける時期なら、
 * サーバーがタスクの作成・予定日の変更のときに作り、予定日を変えると撤回する（docs/decisions/0028）
 */
export function findRepottingAvoid(suggestions: readonly Suggestion[], taskId: string): Suggestion | undefined {
  return suggestions.find((s) => s.taskId === taskId && s.messageKey === 'repotting.avoid' && s.status === 'pending');
}

/**
 * 植え替えの時期の注意の文。本文は予定日（messageParams.date）、理由は時期ごとのキー
 * （`repotting.avoid.{midsummer|midwinter|avoid}.reason`）から引く。知らない時期は、一般的な理由にする
 */
export function repottingAvoidText(suggestion: Suggestion, t: TFunction): { message: string; reason: string } {
  const date = suggestion.messageParams.date;
  const period = /^repotting\.avoid\.(\w+)\.reason$/.exec(suggestion.reasonKey)?.[1];
  return {
    message: t('suggestion.repottingAvoid.message', {
      date: typeof date === 'string' && date !== '' ? formatDay(date) : t('suggestion.repottingAvoid.dueDate'),
    }),
    reason: avoidPeriods.includes(period as AvoidPeriod)
      ? t(`suggestion.repottingAvoid.reason.${period as AvoidPeriod}`)
      : t('suggestion.repottingAvoid.reason.general'),
  };
}

/**
 * 今日の一覧のカードに出す、提案の文（要件9-4・19-4）。知っている提案は本文と理由を、
 * 知らない提案（フェーズ2の天気など、アプリが文言を持たないもの）は種類ごとの一般的な文にする
 */
export function todaySuggestionText(suggestion: Suggestion, t: TFunction): string {
  if (suggestion.messageKey === 'repotting.avoid') {
    const { message, reason } = repottingAvoidText(suggestion, t);
    return `${message}。${reason}`;
  }
  if (suggestion.kind === 'conditional' && suggestion.conditionText) {
    return t('suggestion.today.conditional', { condition: suggestion.conditionText });
  }
  return t('suggestion.today.avoid');
}
