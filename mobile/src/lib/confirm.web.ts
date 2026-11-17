// ブラウザ（Expo の web）用。React Native Web の Alert は何もしないため、ブラウザの確認・通知を使う（開発での確認用）
import type { ConfirmOptions } from './confirm';

export type { ConfirmOptions } from './confirm';

export function confirm({ title, message }: ConfirmOptions): Promise<boolean> {
  return Promise.resolve(window.confirm(message ? `${title}\n\n${message}` : title));
}

export function notify(title: string, message?: string): void {
  window.alert(message ? `${title}\n\n${message}` : title);
}
