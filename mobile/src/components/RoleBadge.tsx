import { useTranslation } from 'react-i18next';

import { normalizeRole, type Role } from '@/features/household/permissions';

import { Badge } from './Badge';

/**
 * ロールのバッジ（screen-inventory.md の RoleBadge、foundations.md の 5章の Badge）。
 * オーナーはアクセントの薄い背景、編集者・閲覧者は控えめな背景にして、オーナーだけを目立たせる
 */
export function RoleBadge({ role }: { role: Role | string }) {
  const { t } = useTranslation();
  const normalized = normalizeRole(role);
  return <Badge label={t(`roles.${normalized}`)} tone={normalized === 'owner' ? 'accent' : 'neutral'} />;
}
