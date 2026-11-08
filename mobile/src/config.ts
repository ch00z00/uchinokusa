// 環境ごとの設定。値は .env（EXPO_PUBLIC_ で始まる変数）から、ビルドのときに埋め込まれる。
// EXPO_PUBLIC_ の変数はアプリに埋め込まれて誰でも読めるので、秘密の値を入れない。

export const config = {
  // API のベースの URL（末尾の /v1 まで）
  apiBaseUrl: process.env.EXPO_PUBLIC_API_BASE_URL ?? 'http://localhost:8080/v1',
  firebase: {
    // Emulator ではどんな値でもよい
    apiKey: process.env.EXPO_PUBLIC_FIREBASE_API_KEY ?? 'fake-api-key',
    projectId: process.env.EXPO_PUBLIC_FIREBASE_PROJECT_ID ?? 'demo-uchinokusa',
    // 設定すると、Firebase Auth は本物ではなく Auth Emulator に接続する（例：localhost:9099）
    authEmulatorHost: process.env.EXPO_PUBLIC_FIREBASE_AUTH_EMULATOR_HOST,
  },
} as const;
