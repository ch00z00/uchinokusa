# uchinokusa API サーバー

## ローカルでの起動
```sh
# リポジトリのルートで、PostgreSQL と Firebase Auth Emulator を起動する
docker compose up -d

# server/ で、環境変数を用意してサーバーを起動する
cp .env.example .env
set -a; source .env; set +a
go run ./cmd/uchinokusa serve

curl -i http://localhost:8080/healthz
```

## サブコマンド
| コマンド | 内容 |
|---|---|
| `serve` | API サーバーを起動する。SIGINT / SIGTERM を受けると、処理中のリクエストを待ってから停止する |
| `migrate` | データベースのマイグレーションを実行する（タスク 1.3 で実装） |
| `seed` | 植物マスタの初期データを投入する（タスク 1.3 で実装） |

## 環境変数
| 名前 | 必須 | 初期値 | 内容 |
|---|---|---|---|
| `DATABASE_URL` | ○ | — | PostgreSQL の接続先 |
| `PORT` | | `8080` | 待ち受けるポート（Cloud Run が値を渡す） |
| `LOG_LEVEL` | | `info` | `debug` / `info` / `warn` / `error` |
| `FIREBASE_PROJECT_ID` | | — | Firebase のプロジェクト ID（ローカルは `demo-uchinokusa`） |
| `FIREBASE_AUTH_EMULATOR_HOST` | | — | 設定すると、Firebase Admin SDK が Emulator に接続する |
| `GOOGLE_APPLICATION_CREDENTIALS` | | — | サービスアカウントの鍵ファイル（Cloud Run では不要） |
| `ALLOWED_ORIGINS` | | — | 許可するオリジン（カンマ区切り） |

## テスト
```sh
go test -race ./...
```
