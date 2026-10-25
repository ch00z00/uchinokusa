# Go のプロジェクトの構成と main.go

- 日付：2026-10-25
- きっかけ：タスク 1.1 で作った `server/cmd/uchinokusa/main.go` の役割

## Q. `server/cmd/uchinokusa/main.go` は、Go においてどんなファイル？

**実行ファイル（`uchinokusa` というコマンド）の入り口** である。プログラムを起動すると、最初にこのファイルの `main()` が呼ばれる。

### Go の決まり：`package main` と `func main()`

Go のパッケージには2種類ある。

| 種類 | 書き方 | ビルドすると |
|---|---|---|
| 実行できるプログラム | `package main` に `func main()` を書く | 実行ファイルになる |
| ライブラリ | `package config` など、main 以外の名前 | 実行ファイルにはならない。ほかのパッケージから import して使う |

`main.go` というファイル名は決まりではない。大事なのは `package main` と `func main()` のほうで、`main.go` は「入り口はここ」と分かるようにするための慣習の名前である。

### 構成の慣習：`cmd/<コマンド名>/`

```
server/
├── go.mod                   モジュール名：github.com/ch00z00/uchinokusa/server
├── cmd/
│   └── uchinokusa/          ← ディレクトリ名が、そのまま実行ファイルの名前になる
│       ├── main.go          package main（入り口）
│       └── main_test.go
└── internal/                ← 中身の処理はこちら（ライブラリのパッケージ）
    ├── config/              環境変数から設定を読む
    ├── logging/             構造化ログ
    └── adapter/httpapi/     ルーター・ハンドラー・ミドルウェア
```

- `cmd/` の下に、実行ファイルごとに1つディレクトリを作るのは、Go でよく使われる慣習である（言語の決まりではない）。あとで別のコマンド（例：`cmd/worker/`）を足しても、並べて置ける
- `go build ./cmd/uchinokusa` を実行すると、ディレクトリ名から `uchinokusa` という実行ファイルができる
- `internal/` は **Go の決まり** で、`internal/` の1つ上のディレクトリ（ここでは `server/`）の中からしか import できない。外のプロジェクトから使われたくない処理を置く場所になる

### `main.go` は薄く保つ

`main.go` には、処理の中身を書かない。`internal/` の部品を組み立てて動かすこと（配線）だけを書く。中身を `internal/` に置くと、パッケージごとにテストしやすくなる。

## Q. このプロジェクトの `main.go` は、どういう流れで動く？

```
$ uchinokusa serve
   |
   v
main()                          Ctrl+C や SIGTERM で終わる ctx を作る
   |
   v
run(ctx, args, getenv, out)
   |-- args[0] でサブコマンドを選ぶ     "serve" -> serve
   |-- config.Load(getenv)            環境変数を読む
   |-- slog.New(logging.NewHandler)   ロガーを作る
   v
serve(ctx, cfg, logger)
   |-- httpapi.NewRouter(logger)      ルーターを作る
   |-- srv.ListenAndServe()           待ち受けを始める
   `-- ctx.Done() -> srv.Shutdown()   止める合図が来たら、処理中のリクエストを待って終了する
```

### なぜ `main()` と `run()` を分けているか

`main()` には、テストしにくい点が2つある。

- 戻り値がない（エラーを返せない）
- 失敗したときに `os.Exit(1)` を呼ぶ。テストの途中で呼ぶと、テスト自体が終了してしまう

そこで、中身を `run()` に移し、`main()` は「`run()` を呼んで、エラーなら終了コード 1 で終わる」だけにしている。

さらに `run()` は、外の世界とのつながりを引数で受け取る。

| 引数 | 本番で渡すもの | テストで渡すもの |
|---|---|---|
| `args` | `os.Args[1:]`（コマンドライン引数） | `[]string{"serve"}` など |
| `getenv` | `os.Getenv`（本物の環境変数） | 決まった値を返す関数 |
| `logOut` | `os.Stdout` | `bytes.Buffer`（出力を確かめられる） |

これで `main_test.go` から、本物の環境変数を書き換えずに、いろいろな条件を試せる。

### `signal.NotifyContext` の役割

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
```

Ctrl+C（`os.Interrupt`）や SIGTERM（Cloud Run が停止のときに送る合図）を受け取ると、`ctx` が終わる（`ctx.Done()` が閉じる）。`serve` はこれを見て、グレースフルシャットダウン（処理中のリクエストを待ってから停止）を始める。

## Q. グレースフルシャットダウンとは？

**止める合図を受けたら、新しいリクエストの受け付けをやめ、処理中のリクエストを最後まで終わらせてから停止すること** である。graceful は「上品な、穏やかな」という意味。

例えるなら、飲食店の閉店のしかた。

- **グレースフル**：入口を閉めて新しいお客さんは入れない。食事中のお客さんには最後まで食べてもらってから、店を閉める
- **いきなり止める**：食事の途中のお客さんも、全員追い出す

### いきなり止めると何が困るか

たとえば「水やりを記録する」リクエストを処理している途中でプロセスが止まると、アプリには接続エラーが返る。データベースはトランザクションで守られるので、中途半端なデータは残らない。それでもユーザーには「記録できたのか、できなかったのか」が分からない。

### いつ止める合図が来るか

- Cloud Run で新しい版をデプロイしたとき（古いインスタンスが止められる）
- アクセスが減って、Cloud Run がインスタンスの数を減らすとき
- ローカルで Ctrl+C を押したとき

Cloud Run は、まず SIGTERM（止めてほしいという合図）を送り、10秒たっても終わらなければ SIGKILL（強制終了）する。

### 時間の流れ

```
時間 ------------------------------------------------------------->
              SIGTERM                                 SIGKILL（10秒後）
                 |                                        |
リクエスト A  ====|===> 完了                               |
リクエスト B    ==|==========> 完了                        |
リクエスト C      |  x 受け付けない（Cloud Run がほかのインスタンスへ送る）
                 |                                        |
                 |<--- Shutdown が待つ（最大8秒） --->|     |
                                                     終了
```

### このプロジェクトのコード（`server/cmd/uchinokusa/main.go` の `serve`）

1. `main()` の `signal.NotifyContext` が、SIGTERM や Ctrl+C を受け取ると `ctx` を終わらせる
2. `serve` は `select` で `ctx.Done()` を待っている。合図が来ると `"shutting down"` をログに出す
3. `srv.Shutdown(shutdownCtx)` を呼ぶ。これは Go の標準の `net/http` の機能で、次のことをする
   - 待ち受けを閉じて、新しい接続を受け付けない
   - 何もしていない接続を閉じる
   - 処理中のリクエストが終わるまで待つ
4. すべて終われば `"server stopped"` を出して、終了コード 0 で終わる

待つ時間（`shutdownTimeout`）を8秒にしているのは、Cloud Run の10秒より短くするため。8秒たっても終わらないリクエストがあれば、`Shutdown` はエラーを返し、強制終了される前に自分で終了する。

### 注意：WebSocket の接続は待たない

`http.Server.Shutdown` は、WebSocket のように HTTP から切り替えた接続（hijack された接続）を、閉じることも待つこともしない。あとで WebSocket を実装するときに、`srv.RegisterOnShutdown` などを使って、自分で接続を閉じる処理を加える必要がある。
