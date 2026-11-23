import AsyncStorage from '@react-native-async-storage/async-storage';

// アプリが端末に保存する値。キーにはすべて接頭辞をつけ、サインアウトでまとめて消せるようにする
// （Firebase のサインインの情報は、Firebase がサインアウトで消す）
const prefix = 'uchinokusa:';

export const storageKeys = {
  /** プッシュ通知の送信先として登録した端末の ID（POST /me/devices の戻り値。サインアウトで解除に使う） */
  deviceId: `${prefix}deviceId`,
  /** 表示する世帯の ID（要件2-4。世帯を切り替えたときに保存し、次の起動でも同じ世帯を出す） */
  selectedHouseholdId: `${prefix}selectedHouseholdId`,
  /** 今日やることの表示（予定日順・置き場所ごと・作業の種類ごと。要件9-5） */
  todayGrouping: `${prefix}todayGrouping`,
} as const;

export async function getDeviceId(): Promise<string | null> {
  return AsyncStorage.getItem(storageKeys.deviceId);
}

export async function setDeviceId(id: string): Promise<void> {
  await AsyncStorage.setItem(storageKeys.deviceId, id);
}

export async function getSelectedHouseholdId(): Promise<string | null> {
  return AsyncStorage.getItem(storageKeys.selectedHouseholdId);
}

export async function setSelectedHouseholdId(id: string): Promise<void> {
  await AsyncStorage.setItem(storageKeys.selectedHouseholdId, id);
}

export async function getTodayGrouping(): Promise<string | null> {
  return AsyncStorage.getItem(storageKeys.todayGrouping);
}

export async function setTodayGrouping(value: string): Promise<void> {
  await AsyncStorage.setItem(storageKeys.todayGrouping, value);
}

/** アプリが保存した値を、すべて消す（サインアウトのとき） */
export async function clearAppStorage(): Promise<void> {
  const keys = await AsyncStorage.getAllKeys();
  await AsyncStorage.multiRemove(keys.filter((key) => key.startsWith(prefix)));
}
