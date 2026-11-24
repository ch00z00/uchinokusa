import { useTranslation } from 'react-i18next';

import { Badge } from './Badge';

/** 期限切れの日数（アプリの層。foundations.md の 5章の OverdueBadge。要件9-3）。「3日過ぎています」 */
/** onTint：目立たせた行（地が accent/subtle）の上に置くとき */
export function OverdueBadge({ days, onTint }: { days: number; onTint?: boolean }) {
  const { t } = useTranslation();
  return <Badge label={t('today.overdue', { count: days })} tone="accent" onTint={onTint} />;
}
