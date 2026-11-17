export type SignOutSteps = {
  getDeviceId: () => Promise<string | null>;
  /** 端末をプッシュ通知の送信先から外す（DELETE /me/devices/{id}） */
  unregisterDevice: (deviceId: string) => Promise<void>;
  signOut: () => Promise<void>;
  /** サーバーのデータのキャッシュを消す */
  clearCache: () => void;
  /** アプリが端末に保存した値を消す */
  clearStorage: () => Promise<void>;
};

/**
 * サインアウトの手順（要件1-5）。
 * 1. 端末を通知の送信先から外す：ID トークンが要るので、サインアウトの前に行う。
 *    失敗しても（通信できないなど）サインアウトは止めない。サインアウトできないと、利用者が困るため
 * 2. サインアウトする
 * 3. 端末上の情報を消す：キャッシュ（ほかの人の世帯のデータが残らないように）と、保存した値
 */
export async function performSignOut(steps: SignOutSteps): Promise<void> {
  const deviceId = await steps.getDeviceId();
  if (deviceId !== null) {
    await steps.unregisterDevice(deviceId).catch(() => undefined);
  }
  await steps.signOut();
  steps.clearCache();
  await steps.clearStorage();
}
