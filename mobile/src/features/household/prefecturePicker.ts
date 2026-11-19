import { router } from 'expo-router';
import { Keyboard } from 'react-native';

// 都道府県の選択（/prefecture のシート）と、開いた入力欄をつなぐ。
// 選んだ値は画面の URL では返せないので、開いた側の受け取り先を覚えておく
let onPick: ((value: string) => void) | null = null;

/** 都道府県の一覧のシートを開く。選んだら onChange が呼ばれる */
export function openPrefecturePicker(current: string, onChange: (value: string) => void): void {
  Keyboard.dismiss();
  onPick = onChange;
  router.push({ pathname: '/prefecture', params: { value: current } });
}

/** シートで選んだ値を、開いた入力欄に返す */
export function pickPrefecture(value: string): void {
  onPick?.(value);
  onPick = null;
}
