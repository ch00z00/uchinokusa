import type { components } from '@/api/schema';
import { careKinds, type CareKind } from '@/features/task/careTask';

export type CareRecord = components['schemas']['CareRecord'];
export type CareRecordPage = components['schemas']['CareRecordPage'];
export type LastPerformed = components['schemas']['LastPerformed'];

/** 履歴の絞り込み（「すべて」と作業の種類。要件12-2） */
export type HistoryFilter = 'all' | CareKind;
export const historyFilters: readonly HistoryFilter[] = ['all', ...careKinds];

/** 絞り込みを API の `kind` にする。「すべて」なら送らない */
export function kindParam(filter: HistoryFilter): CareKind | undefined {
  return filter === 'all' ? undefined : filter;
}

/**
 * 履歴のページをつなぐ。続きを読み込むあいだに新しい記録が入ると、ページの境目がずれて
 * 同じ記録が次のページにも入るので、2回目は出さない
 */
export function flattenRecords(pages: readonly Pick<CareRecordPage, 'records'>[] | undefined): CareRecord[] {
  const seen = new Set<string>();
  const records: CareRecord[] = [];
  for (const page of pages ?? []) {
    for (const r of page.records) {
      if (seen.has(r.id)) continue;
      seen.add(r.id);
      records.push(r);
    }
  }
  return records;
}

/** 記録1件を取る API はないので、取得した履歴（絞り込みごとの、ページの並び）から探す */
export function findRecord(
  results: readonly (readonly Pick<CareRecordPage, 'records'>[] | undefined)[],
  recordId: string,
): CareRecord | undefined {
  for (const pages of results) {
    const found = flattenRecords(pages).find((r) => r.id === recordId);
    if (found) return found;
  }
  return undefined;
}

/**
 * 対応したメンバー（要件12-5）。世帯を抜けただけの人は、記録の時点の名前のまま。
 * アカウントを削除した人は、名前を出さず「退会したユーザー」にする
 */
export function actorOf(record: CareRecord): { status: 'member'; name: string } | { status: 'deleted' } {
  return record.actorUserId === undefined ? { status: 'deleted' } : { status: 'member', name: record.actorDisplayName };
}

/** 記録の日付の確認（要件12-4）。未来の日は選べない */
export function validateRecordDate(date: string, today: string): 'future' | undefined {
  return date > today ? 'future' : undefined;
}
