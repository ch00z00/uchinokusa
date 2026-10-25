# コードリーディング：`server/cmd/uchinokusa/main.go`

- 日付：2026-10-25
- 対象：コミット `1572c75` 時点の `main.go`（Go 1.24）
- 全体の流れと、`main()` と `run()` を分ける理由は [go-project-layout.md](go-project-layout.md) を参照

上から順に、ブロックごとに読む。各見出しの行番号は `main.go` の行。

## 1〜6行目：ファイルの説明と `package main`

```go
// uchinokusa は、API サーバー・マイグレーション・初期データの投入を、サブコマンドで実行する。
//
//	uchinokusa serve    API サーバーを起動する
//	...
package main
```

- `package` の直前のコメントは、パッケージの説明（doc コメント）になる。`go doc` や pkg.go.dev で表示される
- `//` のあとがタブで字下げされた行は、ドキュメント上でコードとして表示される
- `package main` は「これは実行できるプログラムである」という宣言

## 8〜25行目：import

```go
import (
	"context"
	"errors"
	...
	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/config"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)
```

- 使うパッケージを並べる。**使っていないパッケージを import するとコンパイルエラーになる**（Go の決まり）
- 上のまとまりは標準ライブラリ、空行のあとは自分のプロジェクトのパッケージ（慣習として分ける）
- 自分のパッケージのパスは「`go.mod` のモジュール名（`github.com/ch00z00/uchinokusa/server`）＋ディレクトリ」。使うときは最後の部分の名前で呼ぶ（`config.Load`、`httpapi.NewRouter`）

| パッケージ | このファイルでの用途 |
|---|---|
| `context` | 「止めて」という合図や期限を、関数に伝える |
| `errors`、`fmt` | エラーを作る、文字列を組み立てて出力する |
| `io` | 書き込み先を表す `io.Writer` |
| `log/slog` | 構造化ログ |
| `net`、`net/http`、`strconv` | HTTP サーバー、アドレスの組み立て、数値から文字列への変換 |
| `os`、`os/signal`、`syscall` | コマンドライン引数・環境変数・終了、シグナル（Ctrl+C や SIGTERM）の受け取り |
| `time` | 時間（秒数）の表現 |

## 27〜41行目：定数と変数

```go
const usage = `usage: uchinokusa <command>
...`

var (
	errUsage          = errors.New(usage)
	errNotImplemented = errors.New("not implemented yet")
)

const shutdownTimeout = 8 * time.Second
```

- バッククォート（`` ` ``）で囲んだ文字列は、改行をそのまま含められる（raw string）
- `errors.New` で、比べるための目印になるエラーを作っておく（センチネルエラーと呼ぶ）。テストで `errors.Is(err, errUsage)` のように確かめられる
- **名前の先頭が小文字だと、このパッケージの中だけで使える（非公開）**。大文字なら、ほかのパッケージから使える（公開）。`config.Load` が大文字なのはこのため。Go には `public` や `private` というキーワードがなく、名前の大文字・小文字で決まる
- `8 * time.Second` は `time.Duration` 型で「8秒」を表す。数字だけの `8` だと単位が分からないので、こう書く

## 43〜51行目：`main()`

```go
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- **`:=`** は「変数を宣言して、値を入れる」の短い書き方。型は右辺から自動で決まる
- **Go の関数は、値を複数返せる**。`signal.NotifyContext` は `ctx`（合図が来ると終わる context）と `stop`（受け取りをやめる関数）の2つを返す
- **`defer`** は「この関数を抜けるときに実行する」という予約。片付けを、準備のすぐ下に書ける
  - ただし `os.Exit` は `defer` を実行せずに、すぐ終了する。ここでは終了すれば後片付けは不要なので問題ない
- **`if err := ...; err != nil {`** は、「関数を呼んで、エラーがあれば」という Go で一番よく見る形。`err` はこの `if` の中だけで使える
- **Go には例外（try / catch）がない**。エラーは戻り値として返し、呼んだ側が `if err != nil` で確かめる
- `os.Args` はコマンドライン引数。`os.Args[0]` はプログラム自身の名前なので、`[1:]`（1番目から最後まで）で `serve` などだけを渡す
- `os.Getenv` には `()` が付いていない。**関数を呼ぶのではなく、関数そのものを値として渡している**
- エラーは標準エラー出力（`os.Stderr`）に出し、終了コード 1 で終わる。終了コードが 0 以外だと、シェルや Cloud Run は失敗と判断する

## 53〜60行目：サブコマンドの一覧

```go
type command func(ctx context.Context, cfg config.Config, logger *slog.Logger) error

var commands = map[string]command{
	"serve":   serve,
	"migrate": notImplemented("migrate"),
	"seed":    notImplemented("seed"),
}
```

- `type command func(...) error` は、「この形の引数と戻り値を持つ関数」に `command` という名前を付けている（関数型）
- `map[string]command` は、文字列をキーに `command` を引ける表（ほかの言語の辞書・ハッシュ）
- `"serve": serve` は、`serve` 関数そのものを値として登録している
- サブコマンドを増やすときは、この表に1行足すだけでよい
- `*slog.Logger` の `*` はポインタ（値そのものではなく、値の場所）。ロガーのように1つを共有したいものは、ポインタで渡すのが普通

## 62〜79行目：`run()`

```go
func run(ctx context.Context, args []string, getenv func(string) string, logOut io.Writer) error {
	if len(args) == 0 {
		return errUsage
	}
	cmd, ok := commands[args[0]]
	if !ok {
		return errUsage
	}

	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	logger := slog.New(logging.NewHandler(logOut, cfg.LogLevel))

	return cmd(ctx, cfg, logger)
}
```

- `context.Context` を最初の引数にするのは Go の慣習。止める合図や期限を、下の関数へ順に伝えていく
- `[]string` は文字列のスライス（長さを変えられる配列）
- `getenv func(string) string` は「文字列を受け取って文字列を返す関数」を受け取る引数。本番では `os.Getenv`、テストでは決まった値を返す関数を渡す
- `io.Writer` は「書き込める何か」を表すインターフェース。`os.Stdout` もテスト用の `bytes.Buffer` も渡せる
- **`cmd, ok := commands[args[0]]`** は comma ok と呼ばれる形。キーが表にあれば `ok` が `true` になる。ないときは `ok` が `false` で、`cmd` は空（`nil`）
- 問題があればすぐ `return` する（早期リターン）。正常な流れが左端にまっすぐ並び、読みやすくなる
- 引数の確認を、設定の読み込みより先にしている。間違ったコマンドなら、環境変数が足りなくても、使い方の説明を先に返せる
- 最後の `cmd(ctx, cfg, logger)` で、表から取り出した関数を呼ぶ

## 81〜85行目：`notImplemented()`

```go
func notImplemented(name string) command {
	return func(context.Context, config.Config, *slog.Logger) error {
		return fmt.Errorf("%s: %w", name, errNotImplemented)
	}
}
```

- **関数を返す関数** である。返している `func(...) { ... }` は名前のない関数（無名関数）
- 返された関数は、外側の `name` を覚えている（クロージャ）。そのため `notImplemented("migrate")` と `notImplemented("seed")` で、違うメッセージを返す関数ができる
- 引数を使わないので、引数の名前を省略している（型だけ書けばよい）
- **`fmt.Errorf` の `%w`** は、元のエラーを包む（wrap する）書き方。メッセージは `migrate: not implemented yet` になり、`errors.Is(err, errNotImplemented)` で中身を確かめられる。`%v` だと文字列に変わるだけで、元のエラーとのつながりが切れる

## 87〜94行目：`serve()` の HTTP サーバーの準備

```go
srv := &http.Server{
	Addr:              net.JoinHostPort("", strconv.Itoa(cfg.Port)),
	Handler:           httpapi.NewRouter(logger),
	ReadHeaderTimeout: 10 * time.Second,
	ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
}
```

- `http.Server{...}` は構造体（項目をまとめた型）の値を作る書き方。`&` を付けて、そのポインタを `srv` に入れる
- `strconv.Itoa(8080)` は数値を文字列 `"8080"` にする。`net.JoinHostPort("", "8080")` は `":8080"` になり、「このマシンのすべてのアドレスの 8080 番で待つ」という意味
- `Handler` は、リクエストを受け取って処理するもの。ここではルーター（`internal/adapter/httpapi`）
- `ReadHeaderTimeout` は、リクエストのヘッダーを受け取り終えるまでの期限。少しずつ送り続けて接続を占有する攻撃（Slowloris）を防ぐ。`http.Server` の初期値は「期限なし」なので、自分で指定する
- `ErrorLog` は、サーバー内部のエラーの出力先。`http.Server` は古い `log.Logger` の形を求めるので、`slog.NewLogLogger` で変換し、エラーも JSON のログにそろえる

## 96〜100行目：goroutine でサーバーを動かす

図を使った詳しい説明は [go-goroutine.md](go-goroutine.md) を参照。

```go
errCh := make(chan error, 1)
go func() {
	logger.Info("server started", slog.Int("port", cfg.Port))
	errCh <- srv.ListenAndServe()
}()
```

- `srv.ListenAndServe()` は、サーバーが止まるまで戻ってこない。そのまま呼ぶと、止める合図を待つ処理に進めない
- そこで **`go`** を付けて、別の流れ（goroutine：軽いスレッドのようなもの）で動かす。`func() { ... }()` は無名関数を作って、その場で呼んでいる
- **チャネル（`chan`）** は、goroutine 同士で値を受け渡す管。`errCh <- 値` で送り、`<-errCh` で受け取る
- `ListenAndServe` が終わると、そのエラーを `errCh` に送る
- `make(chan error, 1)` の `1` は、受け取る人がいなくても1つ置いておける枠（バッファ）。正常に止めるときは、誰も `errCh` を読まない。枠がないと、送る側の goroutine が永遠に待ち、消えずに残ってしまう（goroutine のリーク）

```
serve（メインの流れ）                      goroutine
   |  go func() --------------------------> ListenAndServe()（リクエストを待ち続ける）
   v                                            |
 select で待つ <-------- errCh -----------------+ 起動に失敗したら、エラーが届く
   ^
   +-------- ctx.Done()（SIGTERM / Ctrl+C）
```

## 102〜107行目：`select` で、先に起きたほうに反応する

```go
select {
case err := <-errCh:
	return fmt.Errorf("serve: %w", err)
case <-ctx.Done():
}
```

- `select` は、複数のチャネルのうち **先に値が届いたほう** の処理を実行する
- `errCh` が先なら、起動に失敗した（ポートが使用中など）。エラーを返して終わる
- `ctx.Done()` が先なら、止める合図が来た。中身が空なので、何もせずに `select` を抜けて、下の停止の処理へ進む

## 109〜117行目：グレースフルシャットダウン

```go
logger.Info("shutting down")
shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
defer cancel()
if err := srv.Shutdown(shutdownCtx); err != nil {
	return fmt.Errorf("shutdown: %w", err)
}
logger.Info("server stopped")
return nil
```

- `context.WithTimeout` で「8秒で期限切れになる context」を作り、`Shutdown` に渡す。8秒たっても処理中のリクエストが終わらなければ、`Shutdown` はあきらめてエラーを返す
- **`ctx` ではなく `context.Background()` から作っているのが大事な点**。`ctx` はすでに終わっている（止める合図で終わった）ので、それをもとにすると、`Shutdown` が1秒も待たずにあきらめてしまう
- `defer cancel()` は、context が使っているタイマーを片付けるため。`WithTimeout` を使ったら、必ず `cancel` を呼ぶのが決まり（`go vet` も呼び忘れを警告する）
- 最後に `nil` を返す。`nil` は「エラーなし」を表す

グレースフルシャットダウンそのものについては、[go-project-layout.md](go-project-layout.md) を参照。

## このファイルに出てきた Go の基本のまとめ

| 書き方 | 意味 |
|---|---|
| `package main` と `func main()` | 実行できるプログラムの入り口 |
| 名前の先頭の大文字・小文字 | 公開・非公開 |
| `:=` | 変数の宣言と代入を一度に |
| 複数の戻り値、`if err != nil` | エラーは戻り値で返し、呼んだ側で確かめる |
| `defer` | 関数を抜けるときに実行する |
| 関数型、関数を値として渡す、クロージャ | 関数も普通の値として扱える |
| `map`、comma ok | 表から引く。あるかどうかを `ok` で確かめる |
| `fmt.Errorf` の `%w`、`errors.Is` | エラーを包み、あとで中身を確かめる |
| `go`、`chan`、`select` | 並行に動かし、値を受け渡し、先に来たものに反応する |
| `context` | 止める合図と期限を伝える |

## Q. 27〜41行目の `usage` は何？

**「このコマンドの使い方」を説明する文章（使い方のメッセージ）** である。usage は英語で「使い方、用法」という意味。

`usage` は Go のキーワードではなく、ただの定数の名前である。ただ、コマンドラインのツールでは、使い方の説明を usage と呼ぶのが昔からの慣習になっている。Go の標準ライブラリの `flag` パッケージにも、使い方を表示する `flag.Usage` がある。

### いつ表示されるか

引数がないときと、知らないサブコマンドを指定したときに表示される。

```
$ go run ./cmd/uchinokusa
usage: uchinokusa <command>

commands:
  serve    start the API server
  migrate  run database migrations
  seed     load the plant species master data
exit status 1

$ go run ./cmd/uchinokusa hello
（同じ説明が表示される）
```

### コードの流れ

1. `const usage = ...` で、説明の文章を定数にする
2. `errUsage = errors.New(usage)` で、その文章をメッセージに持つエラーを作る
3. `run()` は、引数がないときや、`commands` の表にないときに `errUsage` を返す
4. `main()` が、そのエラーを標準エラー出力に出して、終了コード 1 で終わる

使い方の説明をエラーとして返すことで、`main()` は「エラーを表示して終わる」という1つの処理だけで済む。テストでも `errors.Is(err, errUsage)` で、「使い方の説明を返したか」を確かめられる。

### 書き方の慣習

- `<command>` の山かっこは、「ここに何か必ず書く」という意味。省略してよいものは `[--verbose]` のように角かっこで書く
- 一般的なツールでは、`-h` や `--help` を付けたときにも同じ説明を出す。このプロジェクトでは、まだ対応していない（`--help` は知らないサブコマンドとして扱われ、結果的に同じ説明が表示される）

### 確かめた動き

| 実行のしかた | 出力 | 終了コード |
|---|---|---|
| 引数なし | 使い方の説明 | 1 |
| `hello`（知らないコマンド） | 使い方の説明 | 1 |
| `migrate`（`DATABASE_URL` なし） | `invalid config: DATABASE_URL is required` | 1 |
| `migrate`（`DATABASE_URL` あり） | `migrate: not implemented yet` | 1 |

コマンド名が正しいかを、設定の読み込みより前に確かめているので、間違ったコマンドなら、環境変数が足りなくても使い方の説明が先に出る。
