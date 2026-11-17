import { normalizeRole } from './permissions';

/**
 * 世帯から退出できるか（要件2-7、2-8）。サーバーも同じ判定で `409 last_owner` を返すが、押す前に案内する。
 * - allowed：退出できる（確認してから退出する）
 * - transferRequired：唯一のオーナーで、ほかにメンバーがいる。先にオーナー権限を移す
 * - deleteRequired：唯一のオーナーで、自分だけ。退出ではなく世帯を削除する
 */
export type LeaveDecision = 'allowed' | 'transferRequired' | 'deleteRequired';

export function leaveDecision(role: string, memberCount: number): LeaveDecision {
  // オーナーは世帯に1人だけ（交代はオーナー権限の移譲で行う）
  if (normalizeRole(role) !== 'owner') return 'allowed';
  return memberCount > 1 ? 'transferRequired' : 'deleteRequired';
}
