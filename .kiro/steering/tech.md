# 技術スタック

詳しい理由と代替案は `.kiro/specs/household-plant-care-mvp/design.md` の「技術スタック」「重要な設計判断」を参照する。

## 全体の構成
- **モバイル**：React Native + Expo（開発ビルド / EAS Build）。iOS と Android を1つのコードベースで作る
- **API**：Go の1つのバイナリで、REST API・WebSocket・定期ジョブのエンドポイントを提供する（Cloud Run）
- **DB**：PostgreSQL 16（本番は Cloud SQL。コストによっては Neon なども可。PostgreSQL の標準機能だけを使う）
- **マネージドに任せる範囲**：Firebase Auth（認証）、FCM（プッシュ通知）、Firebase Hosting（招待リンクの検証ファイル）、Cloud Scheduler（定期実行）

## モバイル
| 用途 | 採用 |
|---|---|
| フレームワーク | Expo（Expo Go ではなく開発ビルド） |
| 画面遷移・ディープリンク | Expo Router |
| サーバーの状態 | TanStack Query（楽観的更新・キャッシュの無効化） |
| 認証 | `@react-native-firebase/auth` + Apple / Google のサインイン |
| プッシュ通知 | `@react-native-firebase/messaging`（FCM トークン） |
| カレンダー | `react-native-calendars` |
| QRコード | `react-native-qrcode-svg`（表示）、`expo-camera`（読み取り） |
| 多言語 | `i18next` + `expo-localization`（まずは日本語のみ） |
| API の型 | OpenAPI の定義から `openapi-typescript` で生成 |
| テスト | Jest + React Native Testing Library、E2E は Maestro |

## サーバー（Go）
| 用途 | 採用 |
|---|---|
| HTTP | 標準の `net/http` + `oapi-codegen`（OpenAPI から生成） |
| API 定義 | OpenAPI 3.1 を先に書く（`api/openapi.yaml` が唯一の正） |
| WebSocket | `github.com/coder/websocket` |
| DB アクセス | `pgx` + `sqlc` |
| マイグレーション | `goose`（SQL ファイル） |
| Firebase | Firebase Admin SDK for Go（`firebase.google.com/go/v4`）：ID トークンの検証、FCM の送信 |
| ログ | `log/slog`（JSON 形式） |
| テスト | 標準の `testing` + `testcontainers-go`（PostgreSQL） |

## 重要な設計判断（要約）
1. **リアルタイム同期**：WebSocket では「何が変わったか」の通知だけを送り、アプリは REST で取り直す。サーバー間の配信には PostgreSQL の `LISTEN/NOTIFY` を使う（Redis は使わない）
2. **完了の重複の防止**：タスクの予定の回の番号（`cycle_seq`）と、DB の部分ユニーク制約で、同じ回の完了を1件に限る
3. **助言プロバイダ**：提案（実施をすすめる・やらない方がよい・条件つき）は、共通のインターフェースと提案テーブルで扱う。MVPのプロバイダは植え替えのみ

## 制約・注意点
- Cloud Run の WebSocket は最大60分で切れる。アプリは再接続を前提にし、再接続時にデータを取り直す
- Firebase Dynamic Links は終了済み。招待リンクは独自ドメインの Universal Links / App Links で作る
- Sign in with Apple のユーザーを削除するときは、Apple のトークンを失効させる（App Store の審査要件）
- 定期処理はプロセス内のタイマーに頼らず、Cloud Scheduler からジョブのエンドポイントを呼ぶ

## ローカル開発
- Docker Compose で PostgreSQL を起動する
- Firebase Auth Emulator を使い、外部サービスなしで開発・テストする
- 秘密情報（Firebase のサービスアカウント、Apple の秘密鍵）はリポジトリに置かない（本番は Secret Manager）
