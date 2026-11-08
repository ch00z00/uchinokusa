# uchinokusa アプリ（React Native / Expo）

## ローカルでの起動（ブラウザで確かめる）

```sh
# 1. API サーバーと Firebase Auth Emulator を起動する（server/README.md の「ローカルでの起動」）

# 2. mobile/ で、環境変数を用意して、Expo の web を起動する
cp .env.example .env
npm install
npm run web   # http://localhost:8081
```

- サインインの画面の「開発用のユーザーでサインイン」で、Auth Emulator のテストユーザー（`dev@example.com`）でサインインする（いなければ作る）
- スマホの幅で見るには、Chrome の DevTools のデバイスのモード（iPhone など）を使う
  - アプリの中でスマホの幅の枠を作らない。web のヘッダーはウィンドウの幅で計算するため、枠があると表示が崩れる
- ブラウザでは `@react-native-firebase` が動かないので、Firebase の JS SDK を使う（`src/auth/authClient.web.ts`）
- iOS の標準部品は、ブラウザでは見た目が変わる。形と流れはブラウザで試し、最後の見た目は iOS シミュレーターで確かめる
- ルートや設定（`app.json`）を変えて表示がおかしいときは、キャッシュを消して起動し直す：`npx expo start --web -c`

## iOS / Android のビルド

開発ビルド（Expo Go ではない）で動かす。`ios/`・`android/` は `app.json` から生成するので、リポジトリに入れない。

```sh
npm run ios       # ローカル：Xcode が必要
npm run android   # ローカル：Android Studio（Android SDK）が必要
npx eas-cli@latest build --profile development --platform ios   # クラウド（EAS Build）：eas.json
```

## チェック

変更のたびに、型・lint・書式・テストが通ることを確かめる。

```sh
npm run check   # = typecheck + lint + format:check + test
npm run format  # 書式を直す
```

- ロジックとして切り出せるもの（util、フォームのスキーマなど）は、`__tests__/` にユニットテストを書く（Jest）

## 構成

| 場所                   | 中身                                                                                                                          |
| ---------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `src/app/`             | Expo Router の画面。`(auth)` はサインインの前、`(app)` はサインインのあと（各グループの `_layout.tsx` で振り分ける）          |
| `src/api/`             | API のクライアント。`schema.d.ts` は `api/openapi.yaml` から生成した型（手で編集しない。リポジトリのルートで `make gen-api`） |
| `src/auth/`            | 認証。プラットフォームごとに `authClient.web.ts`（ブラウザ）と `authClient.ts`（iOS / Android）に分ける                       |
| `src/query/`           | TanStack Query（サーバーの状態のキャッシュ）の設定                                                                            |
| `src/i18n/`            | 文言（`ja.json`）。画面の文言は直書きせず、`t('...')` で引く                                                                  |
| `src/features/<機能>/` | 機能ごとの部品と、TanStack Query のフック                                                                                     |
| `src/components/`      | 共通の部品（デザインシステムの部品は `.kiro/steering/design.md` の名前にそろえる）                                            |

## API の呼び方

```ts
import { api, unwrap } from '@/api';

const me = await unwrap(api.GET('/me'));
```

- パス・引数・戻り値の型は、生成した型で決まる（存在しないパスや、型の合わない引数はコンパイルエラーになる）
- ID トークンは自動で付く。401 なら、トークンを更新して1回だけ送り直し、それでもだめならサインアウトする
- 失敗は `ApiError` として投げる。画面は `error.code` で分岐し、文言は `t(errorMessageKey(error))` で出す（サーバーの `message` は表示しない）
- 取得（GET）は、通信できないときとサーバーの障害（5xx）のときだけ、自動で再試行する（`src/query/queryClient.ts`）
