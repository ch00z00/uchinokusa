import { Redirect } from 'expo-router';

// 「探す」のタブ（19.1）は画面を持たない。タブを押すと地図アプリを開き、このタブには切り替わらない（(tabs)/_layout.tsx）。
// リンクなどでここへ来たときだけ、今日へ戻す
export default function ShopsTab() {
  return <Redirect href="/today" />;
}
