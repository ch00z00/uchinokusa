import { router } from 'expo-router';

// 植物の登録・編集の「＋ 新しい置き場所」から、置き場所の追加（LOC-02）を開き、作った置き場所を植物の欄に返す。
// 設定の置き場所の一覧から開くときは、返す先がない
let onCreated: ((locationId: string) => void) | null = null;

/** 置き場所の追加を開く。replace なら、今の画面（選択肢のシート）を置き換える */
export function openNewLocation(options: { onCreated?: (locationId: string) => void; replace?: boolean } = {}): void {
  onCreated = options.onCreated ?? null;
  if (options.replace) router.replace('/locations/new');
  else router.push('/locations/new');
}

/** 作った置き場所を、開いた欄に返す */
export function notifyLocationCreated(locationId: string): void {
  onCreated?.(locationId);
  onCreated = null;
}
