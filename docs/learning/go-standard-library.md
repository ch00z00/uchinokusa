# Go の標準ライブラリ

- 日付：2026-10-25
- きっかけ：`main.go` で使っている `os`、`os/signal`、`syscall` の名前の意味

## Q. `os` や `os/signal` の「os」は、Operating System のこと？

そう。`os` は **Operating System（オペレーティングシステム）** の略である。

`go doc os` の説明の冒頭：

> Package os provides a platform-independent interface to operating system functionality.
>
> （os パッケージは、OS の機能を、どの OS でも同じ書き方で使えるようにする）

ファイル、環境変数、コマンドライン引数、プロセスの終了などは、どれもプログラムが OS にお願いして実現している。macOS、Linux、Windows では、お願いのしかた（システムコール）が違う。`os` パッケージがその違いを吸収するので、同じコードがどの OS でも動く。

### 3つのパッケージの関係

```
  あなたのプログラム（main.go）
          |
          v
  +--------------------------------+
  |  os、os/signal                 |   どの OS でも同じ書き方で使える窓口
  +--------------------------------+
          |
          v
  +--------------------------------+
  |  syscall                       |   OS ごとに中身が違う、低いレベルの呼び出し
  +--------------------------------+
          |
          v
  +--------------------------------+
  |  OS のカーネル                  |   macOS / Linux / Windows
  +--------------------------------+
```

- **`os`**：OS の機能を、どの OS でも同じ書き方で使えるようにしたもの。設計は Unix に似せている
- **`os/signal`**：OS から届くシグナルを受け取る
- **`syscall`**：OS ごとの低いレベルの機能（システムコール）をそのまま使う。中身は OS ごとに違う。`os` で足りないときに使う

### シグナルとは

**OS がプロセス（動いているプログラム）に送る、短い合図** である。主に Unix 系（macOS、Linux）の仕組み。

| シグナル | 送られるとき | プログラム側で受け取れるか |
|---|---|---|
| SIGINT | ターミナルで Ctrl+C を押したとき | 受け取れる |
| SIGTERM | 「止まってほしい」と頼むとき（Cloud Run、`kill` コマンドの初期値） | 受け取れる |
| SIGKILL | 強制終了（`kill -9`、Cloud Run が10秒待っても止まらないとき） | 受け取れない（即座に終了する） |

SIGKILL は受け取れないので、後片付けもできない。だから SIGTERM の段階で、グレースフルシャットダウンを済ませておく必要がある。

### `main.go` での使われ方

| 書き方 | パッケージ | 意味 |
|---|---|---|
| `os.Args` | `os` | コマンドライン引数（`serve` など） |
| `os.Getenv` | `os` | 環境変数を読む |
| `os.Stdout`、`os.Stderr` | `os` | 標準出力、標準エラー出力 |
| `os.Exit(1)` | `os` | 終了コードを OS に返して終了する |
| `os.Interrupt` | `os` | Ctrl+C の合図。どの OS でも使える（Windows でも Ctrl+C を表す） |
| `syscall.SIGTERM` | `syscall` | SIGTERM。Unix 系のシグナルなので、`os` ではなく `syscall` にある |
| `signal.NotifyContext` | `os/signal` | 指定したシグナルが届くと終わる context を作る |

`os.Interrupt` が `os` にあり、`SIGTERM` が `syscall` にあるのは、前者がどの OS でも使える一方、後者が Unix 系の仕組みだからである。

## Q. `os/signal` の `/` は何？

ディレクトリの区切りである。`os/signal` は、標準ライブラリの `os` ディレクトリの中にある `signal` ディレクトリのパッケージ。

- import するときはパス全体を書く：`"os/signal"`
- コードの中では、パスの最後の名前で呼ぶ：`signal.NotifyContext`（`os.signal.NotifyContext` ではない）
- `os` と `os/signal` は、別々のパッケージである。`os` を import しても、`os/signal` は使えない

`net/http`（`http.Server`）や `log/slog`（`slog.New`）も同じ仕組み。自分のパッケージの `internal/adapter/httpapi` を `httpapi.NewRouter` と呼ぶのも同じ。

## Q. 止める・終わらせる以外のシグナルもある？

ある。シグナルは「止まれ」専用ではなく、**OS がプロセスに伝える、いろいろな出来事の合図** である。macOS の `kill -l` では31種類が表示される。

```
HUP INT QUIT ILL TRAP ABRT EMT FPE KILL BUS SEGV SYS PIPE ALRM TERM URG STOP TSTP CONT CHLD TTIN TTOU IO XCPU XFSZ VTALRM PROF WINCH INFO USR1 USR2
```

（`kill` コマンドは、名前に反して「シグナルを送る」コマンドである。初期値が SIGTERM なので、止める用途で使われることが多い）

### 種類ごとの代表例

| 種類 | シグナル | いつ送られるか・何に使うか |
|---|---|---|
| 終了 | SIGINT、SIGTERM、SIGKILL | 前の表を参照 |
| 終了（調査用） | SIGQUIT | Ctrl+\ で送られる。Go のプログラムは、**すべての goroutine の状態（スタック）を表示して** 終了する |
| 一時停止と再開 | SIGSTOP、SIGCONT | 一時停止と再開。SIGSTOP は受け取れない（無視できない） |
| 一時停止（ジョブ制御） | SIGTSTP | Ctrl+Z で送られる。`fg` で再開できる |
| 設定の読み直し | SIGHUP | もとは「端末の回線が切れた」という合図。今は、常駐するプログラムに「設定ファイルを読み直して」と伝える慣習によく使われる（nginx など） |
| 自由に使ってよい | SIGUSR1、SIGUSR2 | 意味が決まっていない。プログラムごとに「ログを開き直す」などの意味を決めて使う |
| 状態の変化の通知 | SIGCHLD | 子プロセスが終わった |
| 状態の変化の通知 | SIGWINCH | ターミナルの大きさが変わった（vim などが画面を描き直す） |
| 状態の変化の通知 | SIGPIPE | 相手がすでに閉じた管（パイプ）に書き込もうとした |
| プログラムの誤り | SIGSEGV、SIGBUS | 触ってはいけないメモリに触った |
| プログラムの誤り | SIGFPE | 0 で割るなどの、計算の誤り |
| タイマー | SIGALRM | 指定した時間がたった |

### Go のプログラムでの初期の動き

`go doc os/signal` の「Default behavior of signals in Go programs」より。

| シグナル | 何もしないときの Go の動き |
|---|---|
| SIGHUP、SIGINT、SIGTERM | 終了する |
| SIGQUIT、SIGABRT など | スタックを表示して終了する |
| SIGSEGV などのプログラムの誤り | panic（Go の実行時エラー）に変わる。nil のポインタを触ったときの `invalid memory address or nil pointer dereference` がこれ |
| SIGTSTP など | OS の初期の動き（シェルのジョブ制御のため） |
| SIGPROF | Go のランタイムが、CPU のプロファイリングに使う |
| それ以外（SIGUSR1 など） | 受け取るが、何もしない |

`signal.NotifyContext` や `signal.Notify` で受け取るように指定したシグナルは、この初期の動きの代わりに、プログラムに届く。`main.go` では SIGINT と SIGTERM を受け取るように指定しているので、すぐ終了せずに、グレースフルシャットダウンができる。

### 実際に試した（このプロジェクトのサーバーで）

`uchinokusa serve` を起動して、シグナルを順に送った。

| 送ったシグナル | 結果 |
|---|---|
| SIGSTOP | プロセスが一時停止し、`/healthz` が応答しなくなった（curl がタイムアウト） |
| SIGCONT | 再開し、`/healthz` がまた `{"status":"ok"}` を返した |
| SIGUSR1 | 何も起きなかった（受け取るが、何もしない） |
| SIGQUIT | すべての goroutine の状態を表示して終了した |

SIGQUIT で表示された内容の一部：

```
SIGQUIT: quit

goroutine 1 [select]:
main.serve(...)
	.../server/cmd/uchinokusa/main.go:102
main.run(...)
	.../server/cmd/uchinokusa/main.go:78
main.main()
	.../server/cmd/uchinokusa/main.go:47

goroutine 37 [IO wait]:
net/http.(*Server).Serve(...)
net/http.(*Server).ListenAndServe(...)
main.serve.func1()
	.../server/cmd/uchinokusa/main.go:99
	.../server/cmd/uchinokusa/main.go:97   （この goroutine を起動した場所）
```

[go-goroutine.md](go-goroutine.md) の図のとおりになっている。

- **goroutine 1**（メインの流れ）：`main` → `run` → `serve` と呼ばれ、102行目の `select` で待っている
- **goroutine 37**（97行目の `go func()` で起動した goroutine）：99行目の `ListenAndServe` の中で、新しい接続を待っている（IO wait）

SIGQUIT は、サーバーが固まったときに「どの goroutine が、どこで止まっているか」を調べるのに使える。
