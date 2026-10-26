# uchinokusa API サーバー

## ローカルでの起動
```sh
# リポジトリのルートで、PostgreSQL と Firebase Auth Emulator を起動する
docker compose up -d

# server/ で、環境変数を用意し、マイグレーションを適用してからサーバーを起動する
cp .env.example .env
set -a; source .env; set +a
go run ./cmd/uchinokusa migrate
go run ./cmd/uchinokusa serve

curl -i http://localhost:8080/healthz
```

## サブコマンド
| コマンド | 内容 |
|---|---|
| `serve` | API サーバーを起動する。SIGINT / SIGTERM を受けると、処理中のリクエストを待ってから停止する |
| `migrate` | まだ適用していないマイグレーションをすべて適用する（適用済みなら何もしない） |
| `seed` | 植物マスタの初期データを投入する（タスク 6.1 で実装） |

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

## API 定義からのコードの生成
API 定義は `api/openapi.yaml` が唯一の正である（`docs/decisions/0001`）。定義を変えたら、リポジトリのルートで生成し直す。
```sh
make gen-api     # サーバー（internal/adapter/httpapi/oapi/oapi.gen.go）とアプリの型を生成する
make check-gen   # 生成したファイルが定義と一致しているかを確かめる
```
- 生成ツール（oapi-codegen）は `go.mod` の `tool` で管理しているため、別途インストールは要らない
- エラーは `internal/domain` のエラーを返せば、`internal/adapter/httpapi/errors.go` で HTTP のステータスと `code` に変換される

## データベース
- マイグレーション：`db/migrations/`（goose の SQL ファイル）。ファイルはバイナリに埋め込まれる（`db/db.go`）
- クエリ：`db/queries/`（sqlc の SQL ファイル）。変えたら、リポジトリのルートで `make gen-db` を実行する
  - 生成先：`internal/adapter/postgres/sqlcgen/`（手で編集しない）
  - sqlc は `go.mod` の `tool` で管理している。初回の `make gen-db` は、sqlc のビルドに1分ほどかかる
- 世帯の配下のテーブルは、親への外部キーを `(id, household_id)` の組で張っている。別の世帯のデータを参照する行は、DB が拒否する

## テスト
```sh
go test -race ./...         # 結合テストを含む（Docker が必要）
go test -race -short ./...  # 結合テストを飛ばす
```
- 結合テストは、`internal/adapter/postgres/pgtest` が testcontainers で PostgreSQL を起動する。テストごとに、マイグレーション済みのテンプレートから新しいデータベースを複製するため、並行に実行できる
