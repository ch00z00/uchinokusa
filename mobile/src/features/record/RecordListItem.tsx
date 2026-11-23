import { useTranslation } from 'react-i18next';

import { CareKindIcon } from '@/components/CareKindIcon';
import { ListRow } from '@/components/ListRow';
import { normalizeCareKind } from '@/features/task/careTask';
import { taskTitle } from '@/features/task/taskText';
import { formatDay } from '@/lib/formatDate';

import { actorOf, type CareRecord } from './careRecord';

type Props = {
  record: CareRecord;
  /** 押すと記録の編集（PLANT-12。編集者以上）。渡さなければ押せない */
  onPress?: () => void;
};

/**
 * 履歴の記録（foundations.md の 5章の RecordListItem。PLANT-11、要件12-1・12-5）。
 * 補足の1行目に「完了」または「スキップ」と実施日、2行目に対応したメンバー。完了とスキップは色ではなく言葉で区別する
 */
export function RecordListItem({ record, onPress }: Props) {
  const { t } = useTranslation();
  const separator = t('tasks.list.separator');
  const actor = actorOf(record);
  const lines = [
    [
      t(`history.outcome.${record.outcome}`),
      formatDay(record.performedOn),
      record.isExtra ? t('history.extra') : undefined,
    ]
      .filter(Boolean)
      .join(separator),
    actor.status === 'member' ? actor.name : t('history.deletedUser'),
    record.skipReason
      ? t('history.skipReason', {
          reason:
            record.skipReason === 'other' && record.skipReasonText
              ? record.skipReasonText
              : t(`today.skip.reasons.${record.skipReason}`),
        })
      : undefined,
  ];
  return (
    <ListRow
      leading={<CareKindIcon kind={normalizeCareKind(record.kind)} size="lg" />}
      title={taskTitle(record, t)}
      subtitle={lines.filter(Boolean).join('\n')}
      navigates={onPress !== undefined}
      onPress={onPress}
    />
  );
}
