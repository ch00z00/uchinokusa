// Firebase の React Native 用の関数の型。
// 実行時は、Metro が package.json の react-native の条件で React Native 用のビルドを選ぶので、関数はある。
// ただ、TypeScript は先頭の types（共通の型）を選ぶため、React Native 用の関数の型が見えない。
// 宣言は node_modules/@firebase/auth/dist/rn/src/platform_react_native/persistence/react_native.d.ts と同じ
import type { Persistence, ReactNativeAsyncStorage } from 'firebase/auth';

declare module 'firebase/auth' {
  export function getReactNativePersistence(storage: ReactNativeAsyncStorage): Persistence;
}
