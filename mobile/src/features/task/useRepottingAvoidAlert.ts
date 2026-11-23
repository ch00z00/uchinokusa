import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

import { fetchSuggestions, suggestionKeys } from '@/features/suggestion/useSuggestions';
import { findRepottingAvoid, repottingAvoidText } from '@/features/suggestion/suggestionText';
import { notify } from '@/lib/confirm';

import { normalizeCareKind, type CareTask } from './careTask';

/**
 * 植え替えのタスクを保存したあとに、予定日が植え替えに向かない時期なら、OS のアラートで知らせる（要件17-8。16.3c）。
 * 判定はサーバーが保存のときに行い、提案として作る（docs/decisions/0028）。ここでは提案を取り直して探すだけ。
 * 保存は止めない。取り直しに失敗したら、何も出さない（詳細の一覧の注意の行で、あとから見られる）
 */
export function useRepottingAvoidAlert(householdId: string) {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  return async (task: Pick<CareTask, 'id' | 'kind'>) => {
    if (normalizeCareKind(task.kind) !== 'repotting') return;
    let suggestions;
    try {
      suggestions = await queryClient.fetchQuery({
        queryKey: suggestionKeys.list(householdId),
        queryFn: () => fetchSuggestions(householdId),
        staleTime: 0,
      });
    } catch {
      return;
    }
    const avoid = findRepottingAvoid(suggestions, task.id);
    if (!avoid) return;
    const { message, reason } = repottingAvoidText(avoid, t);
    notify(t('suggestion.repottingAvoid.title'), `${message}。${reason}\n\n${t('suggestion.repottingAvoid.saved')}`);
  };
}
