import AsyncStorage from '@react-native-async-storage/async-storage';

// アプリが端末に保存する値。キーにはすべて接頭辞をつけ、サインアウトでまとめて消せるようにする
// （Firebase のサインインの情報は、Firebase がサインアウトで消す）
const prefix = 'uchinokusa:';

export const storageKeys = {
  /** プッシュ通知の送信先として登録した端末の ID（POST /me/devices の戻り値。サインアウトで解除に使う） */
  deviceId: `${prefix}deviceId`,
} as const;

export async function getDeviceId(): Promise<string | null> {
  return AsyncStorage.getItem(storageKeys.deviceId);
}

export async function setDeviceId(id: string): Promise<void> {
  await AsyncStorage.setItem(storageKeys.deviceId, id);
}

/** アプリが保存した値を、すべて消す（サインアウトのとき） */
export async function clearAppStorage(): Promise<void> {
  const keys = await AsyncStorage.getAllKeys();
  await AsyncStorage.multiRemove(keys.filter((key) => key.startsWith(prefix)));
}
