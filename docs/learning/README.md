# 学習ノート

開発中に出てきた技術的な疑問と、その答えを、話題ごとに1ファイルで残す。判断の記録（`docs/decisions/`）とは違い、「何を決めたか」ではなく「どういう仕組みか」をまとめる。

- ファイル名：`話題.md`（例：`docker-compose.md`）
- 同じ話題の質問は、同じファイルに追記する

## 一覧
| ファイル | 内容 |
|---|---|
| [docker-compose.md](docker-compose.md) | `compose.yaml` という名前、Compose Specification、`version` の廃止、`depends_on`、Docker Swarm |
| [go-project-layout.md](go-project-layout.md) | `package main` と `cmd/`・`internal/` の構成、`main.go` の流れ、`main()` と `run()` を分ける理由、グレースフルシャットダウン |
| [code-reading-main-go.md](code-reading-main-go.md) | `server/cmd/uchinokusa/main.go` を上から1ブロックずつ読む（Go の基本の書き方） |
| [go-standard-library.md](go-standard-library.md) | `os` は Operating System、`os`・`os/signal`・`syscall` の関係、シグナル（終了以外のものも含む、実際に送って確かめた結果）、import パスの `/` |
| [go-goroutine.md](go-goroutine.md) | goroutine・チャネル・select を図で説明（`serve` の 96〜107行目）、JavaScript との比較 |
| [openapi.md](openapi.md) | `api/openapi.yaml` に今定義されているもの、`oapi/` のコード生成（oapi-codegen、strict-server） |
| [db-migration.md](db-migration.md) | マイグレーションとは何か、解決するペイン、`00001_init.sql` を作るタイミング、PostgreSQL での注意点（ロック、CONCURRENTLY、expand / contract）、シーディングのタイミングと冪等性 |
| [db-queries-sqlc.md](db-queries-sqlc.md) | `db/migrations/` と `db/queries/` の違い、sqlc による型つきの Go のコードの生成、GraphQL Code Generator との比較 |
