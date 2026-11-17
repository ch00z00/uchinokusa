import type { components } from '@/api/schema';

export type Role = components['schemas']['Role'];

/** 知らないロールは閲覧者として扱う（API の Role の説明。あとからロールが増えても、操作を出しすぎない） */
export function normalizeRole(role: string): Role {
  return role === 'owner' || role === 'editor' ? role : 'viewer';
}

/**
 * アプリで出し分ける操作（screen-inventory.md の 6.1）。
 * - record：完了・スキップ・取り消しなどの記録
 * - edit：植物・置き場所・タスク・メモの作成・編集・削除、提案の採用・見送り
 * - manageHousehold：世帯名・地域の変更、招待、メンバーの管理、オーナー権限の移譲、世帯の削除
 * 退出はロールだけでは決まらない（オーナーはメンバーの数による）ので、leave.ts で判定する
 */
export type Permission = 'record' | 'edit' | 'manageHousehold';

const allowed: Record<Role, readonly Permission[]> = {
  owner: ['record', 'edit', 'manageHousehold'],
  editor: ['record', 'edit'],
  viewer: [],
};

export function can(role: string, permission: Permission): boolean {
  return allowed[normalizeRole(role)].includes(permission);
}
