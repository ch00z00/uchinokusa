import { focusManager } from '@tanstack/react-query';
import { useEffect } from 'react';
import { AppState, Platform } from 'react-native';

/**
 * アプリが前面に戻ったら、TanStack Query に「画面に戻った」と伝え、古いデータを取り直させる。
 * ブラウザでは TanStack Query がタブの表示の切り替えを自分で見ているので、何もしない。
 */
export function useAppStateFocus(): void {
  useEffect(() => {
    if (Platform.OS === 'web') return;
    const subscription = AppState.addEventListener('change', (status) => {
      focusManager.setFocused(status === 'active');
    });
    return () => subscription.remove();
  }, []);
}
