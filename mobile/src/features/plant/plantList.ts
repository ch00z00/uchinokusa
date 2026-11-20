import type { components } from '@/api/schema';

export type Plant = components['schemas']['Plant'];

/** 一覧の並びと絞り込みに使う項目 */
export type PlantListEntry = Pick<Plant, 'id' | 'name' | 'nextDueOn' | 'locationId'>;

/** 置き場所の絞り込み（PLANT-01）。API には「置き場所未設定」の条件がないので、アプリの中で絞る */
export type LocationFilter = { kind: 'all' } | { kind: 'unset' } | { kind: 'location'; id: string };

const byName = (a: PlantListEntry, b: PlantListEntry) => a.name.localeCompare(b.name, 'ja');

/**
 * 次の予定日が近い順に並べる（2026-11-20 に決定。一覧は植物のダッシュボードなので、お世話が近いものを上に）。
 * 予定のタスクがない植物は最後にし、予定日が同じものとあわせて名前順にする
 */
export function sortPlantsByNextDue<T extends PlantListEntry>(plants: readonly T[]): T[] {
  return [...plants].sort((a, b) => {
    if (a.nextDueOn !== b.nextDueOn) {
      if (a.nextDueOn === undefined) return 1;
      if (b.nextDueOn === undefined) return -1;
      // YYYY-MM-DD は文字列の順が日付の順
      return a.nextDueOn < b.nextDueOn ? -1 : 1;
    }
    return byName(a, b);
  });
}

export function filterPlantsByLocation<T extends PlantListEntry>(plants: readonly T[], filter: LocationFilter): T[] {
  switch (filter.kind) {
    case 'all':
      return [...plants];
    case 'unset':
      return plants.filter((p) => p.locationId === undefined);
    case 'location':
      return plants.filter((p) => p.locationId === filter.id);
  }
}
