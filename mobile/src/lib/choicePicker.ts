import { router } from 'expo-router';
import { Keyboard } from 'react-native';

export type Choice = { value: string; label: string };

export type ChoiceRequest = {
  /** シートの題名（欄の見出しと同じにする） */
  title: string;
  options: readonly Choice[];
  value: string;
  onChange: (value: string) => void;
  /** 選択肢の最後に置く操作（「＋ 新しい置き場所」など）。押すと、シートをその画面に置き換える */
  action?: { label: string; onPress: () => void };
};

// 選択肢のシート（/choice）と、開いた欄をつなぐ。
// 選択肢と選んだ値は画面の URL では渡しにくいので、開いた側の依頼を覚えておく
let request: ChoiceRequest | null = null;

/** 選択肢のシートを開く。選んだら onChange が呼ばれる */
export function openChoicePicker(next: ChoiceRequest): void {
  Keyboard.dismiss();
  request = next;
  router.push('/choice');
}

/** シートに出す依頼。アプリを開き直したあとなど、依頼がなければ null */
export function currentChoiceRequest(): ChoiceRequest | null {
  return request;
}

/** シートで選んだ値を、開いた欄に返す */
export function pickChoice(value: string): void {
  request?.onChange(value);
  request = null;
}
