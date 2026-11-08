import { Redirect } from 'expo-router';

// アプリを開いたら「今日」のタブを出す（未サインインなら、サインインの画面に戻される）
export default function Index() {
  return <Redirect href="/today" />;
}
