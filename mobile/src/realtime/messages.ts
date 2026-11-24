/**
 * WebSocket のメッセージ（docs/decisions/0030 の取り決め）と、受け取ったときの判断。画面から切り離した純粋な関数
 */

export type ChangedMessage = {
  type: 'changed';
  householdId: string;
  version: number;
  entity: string;
  entityId: string;
  op: string;
  /** 操作した人。ジョブの書き込みでは、項目がない */
  actorUserId?: string;
  /** 植物に属するデータ（タスク・記録・メモ・提案など）のとき */
  plantId?: string;
};

export type RealtimeMessage =
  | { type: 'subscribed'; householdIds: string[] }
  | ChangedMessage
  | { type: 'membership_revoked'; householdId: string }
  | { type: 'role_changed'; householdId: string; role: string }
  | { type: 'pong' };

const isString = (v: unknown): v is string => typeof v === 'string';

/** サーバーからのメッセージを読む。読めない・知らない種類は null（無視する。新しいサーバーとの組み合わせのため） */
export function parseMessage(raw: string): RealtimeMessage | null {
  let data: unknown;
  try {
    data = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof data !== 'object' || data === null) return null;
  const m = data as Record<string, unknown>;
  switch (m.type) {
    case 'subscribed':
      return Array.isArray(m.householdIds) && m.householdIds.every(isString)
        ? { type: 'subscribed', householdIds: m.householdIds }
        : null;
    case 'changed':
      if (
        !isString(m.householdId) ||
        typeof m.version !== 'number' ||
        !isString(m.entity) ||
        !isString(m.entityId) ||
        !isString(m.op)
      ) {
        return null;
      }
      return {
        type: 'changed',
        householdId: m.householdId,
        version: m.version,
        entity: m.entity,
        entityId: m.entityId,
        op: m.op,
        ...(isString(m.actorUserId) ? { actorUserId: m.actorUserId } : {}),
        ...(isString(m.plantId) ? { plantId: m.plantId } : {}),
      };
    case 'membership_revoked':
      return isString(m.householdId) ? { type: 'membership_revoked', householdId: m.householdId } : null;
    case 'role_changed':
      return isString(m.householdId) && isString(m.role)
        ? { type: 'role_changed', householdId: m.householdId, role: m.role }
        : null;
    case 'pong':
      return { type: 'pong' };
    default:
      return null;
  }
}

/**
 * 世帯のバージョン番号の確かめ（設計の「配信」「順序」）。
 * - apply：受け取って取り直す（はじめての番号か、次の番号）
 * - stale：古い番号なので無視する
 * - gap：2つ以上飛んだので、取りこぼしがある。世帯のデータをすべて取り直す
 */
export function checkVersion(last: number | undefined, version: number): 'apply' | 'stale' | 'gap' {
  if (last === undefined) return 'apply';
  if (version <= last) return 'stale';
  return version === last + 1 ? 'apply' : 'gap';
}

export type InvalidationTarget = { queryKey: readonly string[]; exact?: boolean };

/**
 * 変更イベントで取り直すクエリ（キーの形は features の各 *Keys）。
 * 世帯の取得（今日やること・植物の一覧・提案・置き場所・メンバー）は世帯のキーの下にあるのでまとめて取り直す。
 * 植物の詳細（タスク・履歴・メモ・カレンダー）は植物のキーの下にあるので、植物がわかれば、その植物だけを取り直す。
 * 画面に出ていないクエリは古い印を付けるだけで、開いたときに取り直される
 */
export function invalidationTargets(event: ChangedMessage): InvalidationTarget[] {
  const targets: InvalidationTarget[] = [{ queryKey: ['households', event.householdId] }];
  if (event.entity === 'plant') targets.push({ queryKey: ['plants', event.entityId] });
  else if (event.plantId) targets.push({ queryKey: ['plants', event.plantId] });
  // 置き場所の名前・削除（植物が「置き場所未設定」になる）は、どの植物の詳細にも出る。
  // メンバーの削除（アカウントの削除）では、記録・メモの作業者名が「退会したユーザー」に変わる（イベントは出ない。0035）
  if (event.entity === 'location' || (event.entity === 'member' && event.op === 'deleted')) {
    targets.push({ queryKey: ['plants'] });
  }
  // 世帯の名前とロールは、世帯の一覧（GET /households）から出している
  if (event.entity === 'member' || event.entity === 'household')
    targets.push({ queryKey: ['households'], exact: true });
  return targets;
}

/** つなぎ直すまでの待ち時間（ミリ秒）。1秒から倍にしながら、30秒まで（attempt は 0 から） */
export function reconnectDelay(attempt: number): number {
  return Math.min(1000 * 2 ** attempt, 30_000);
}
