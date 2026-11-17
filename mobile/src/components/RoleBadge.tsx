import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { normalizeRole, type Role } from '@/features/household/permissions';
import { useTheme } from '@/theme';

import { AppText } from './AppText';

/**
 * ロールのバッジ（screen-inventory.md の RoleBadge、foundations.md の 5章の Badge）。
 * オーナーはアクセントの薄い背景、編集者・閲覧者は控えめな背景にして、オーナーだけを目立たせる
 */
export function RoleBadge({ role }: { role: Role | string }) {
  const { t } = useTranslation();
  const theme = useTheme();
  const normalized = normalizeRole(role);
  const owner = normalized === 'owner';
  return (
    <View
      style={{
        alignSelf: 'flex-start',
        paddingHorizontal: theme.space.sm,
        paddingVertical: theme.space.xs,
        borderRadius: theme.radius.sm,
        backgroundColor: owner ? theme.color.accent.subtle : theme.color.bg.subtle,
      }}
    >
      <AppText variant="caption" color={owner ? 'accent' : 'secondary'}>
        {t(`roles.${normalized}`)}
      </AppText>
    </View>
  );
}
