# プロジェクトの構成

まだ実装前のため、以下は**予定の構成**である。実装が進んだら `/kiro:steering` で実態に合わせて更新する。

## リポジトリ（モノレポ）
```
uchinokusa/
├── CLAUDE.md                 Claude Code 向けの開発ルール
├── HANDOFF.md                作業の引き継ぎメモ（現在の状況と次にやること）
├── .kiro/
│   ├── steering/             プロダクト・技術・構成の方針（このファイルを含む）
│   └── specs/                機能ごとの仕様（要件・設計・タスク）
├── api/
│   └── openapi.yaml          API 定義（サーバーとアプリの型の唯一の正）
├── server/                   Go の API サーバー
└── mobile/                   React Native（Expo）のアプリ
```

## サーバー（`server/`）
レイヤーを分けた構成とする。ドメイン層は外部に依存しない。
```
server/
├── cmd/uchinokusa/           エントリーポイント（serve / migrate / seed のサブコマンド）
├── internal/
│   ├── domain/               エンティティ、値オブジェクト、次回予定の計算、季節の判定（純粋なロジック）
│   ├── app/                  ユースケース（各 Service）、権限の判定、トランザクションの境界
│   ├── advisor/              助言プロバイダの取り決め、レジストリ、植え替えのプロバイダ
│   ├── adapter/
│   │   ├── http/             生成したハンドラーの実装、ミドルウェア（認証など）
│   │   ├── ws/               WebSocket の接続管理
│   │   ├── postgres/         sqlc で生成したコード、リポジトリ、LISTEN/NOTIFY
│   │   ├── firebase/         ID トークンの検証、FCM の送信
│   │   └── apple/            Sign in with Apple のトークン失効
│   └── job/                  リマインダー、完了通知、季節の切り替え、日次の助言
└── db/
    ├── migrations/           goose の SQL ファイル
    ├── queries/              sqlc の SQL ファイル
    └── seeds/                植物マスタの初期データ（YAML、出典つき）
```

**依存の向き**：`adapter` → `app` → `domain`。`domain` は他の層を参照しない。

## モバイル（`mobile/`）
```
mobile/
├── app/                      Expo Router の画面（(auth)、(tabs)/today、(tabs)/plants、plants/[id]、settings、invite/[token]）
└── src/
    ├── api/                  生成した型と fetch のラッパー
    ├── features/<機能>/      画面の部品と TanStack Query のフック
    ├── realtime/             WebSocket の接続と、キャッシュの無効化
    ├── auth/                 Firebase Auth、Apple / Google のサインイン
    ├── notifications/        FCM トークンの登録、通知からの画面遷移
    ├── i18n/                 i18next の設定と ja.json
    └── lib/                  小さなユーティリティ（地図アプリの起動など）
```

## 命名と規約
- **Go**：パッケージ名は小文字の単語1つ。エラーはドメインのエラー型に正規化し、HTTP 層の一か所で変換する
- **API**：ベースパスは `/v1`。JSON の項目名はキャメルケース。日付は `YYYY-MM-DD`、日時は RFC 3339（UTC）
- **互換性**：API は項目の追加だけを許可する。アプリは知らない項目・列挙値を無視する
- **DB**：テーブル名・列名はスネークケースの複数形。主キーは UUIDv7。世帯配下のテーブルには `household_id` を持たせ、クエリには必ず世帯の条件を含める
- **TypeScript**：`any` を使わない。画面の文言は直書きせず、i18n のキーから引く
- **テスト**：ドメインのロジックは単体テスト、DB や並行処理が絡むものは testcontainers による結合テストで確認する
