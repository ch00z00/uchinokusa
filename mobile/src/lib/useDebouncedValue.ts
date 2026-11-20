import { useEffect, useState } from 'react';

/** 値の変化が delayMs だけ止まってから、新しい値を返す（検索の入力のたびに API を呼ばないように） */
export function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);
  return debounced;
}
