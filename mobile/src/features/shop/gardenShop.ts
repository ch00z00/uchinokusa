/**
 * 近くの園芸店を探す（タスク 19.1、要件18）。端末の地図アプリで検索結果を開き、開けなければブラウザの地図サービスで開く。
 * 検索は地図アプリが自分の位置情報で行うので、このアプリは位置情報の許可を求めない（要件18-3）
 */

/** 開く URL を、試す順に並べる。地図アプリが「近くの」結果を出す */
export function gardenShopUrls(platform: string, query: string): string[] {
  const q = encodeURIComponent(query);
  const browser = `https://www.google.com/maps/search/?api=1&query=${q}`;
  if (platform === 'ios') return [`maps://?q=${q}`, browser];
  if (platform === 'android') return [`geo:0,0?q=${q}`, browser];
  return [browser];
}

/**
 * 順に開いてみる。開けたら true。地図アプリが消されているなどで開けなければ、次を試す。
 * canOpenURL は、iOS では調べる URL の種類を Info.plist に並べる必要があるので使わず、開いてみて確かめる
 */
export async function openGardenShopSearch(
  platform: string,
  query: string,
  open: (url: string) => Promise<unknown>,
): Promise<boolean> {
  for (const url of gardenShopUrls(platform, query)) {
    try {
      await open(url);
      return true;
    } catch {
      // 次を試す
    }
  }
  return false;
}
