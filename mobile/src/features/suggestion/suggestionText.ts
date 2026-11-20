import type { TFunction } from 'i18next';

import type { components } from '@/api/schema';

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
