# goroutine・チャネル・select

- 日付：2026-10-25
- きっかけ：`server/cmd/uchinokusa/main.go` の 96〜107行目（`serve` の中で、サーバーを goroutine で動かしている部分）

## Q. goroutine とは？

**関数を「別の流れ」で動かし、呼んだ側は終わりを待たずに次へ進めるようにする仕組み** である。`go` というキーワードを関数の呼び出しの前に付けるだけで使える（`goroutine()` という関数があるわけではない）。

```go
doWork()      // 普通の呼び出し：doWork が終わるまで、次の行に進まない
go doWork()   // goroutine：doWork を別の流れで始めて、すぐ次の行に進む
```

### なぜ `serve` で goroutine が必要か

`srv.ListenAndServe()` は、**サーバーが止まるまで戻ってこない** 関数である。リクエストを受け付け続けるのが仕事なので、終わらないのが正常。

goroutine を使わずに書くと、こうなる：

```
serve()
  |
  v
srv.ListenAndServe()      <- ここで止まったまま（サーバーが止まるまで戻ってこない）
  |
  x                          ここから下に進めない
  v
「止める合図を待つ」処理      <- 永遠に実行されない
```

`go` を付けると、2つの流れが同時に進む：

```
     serve（メインの流れ）                     goroutine（別の流れ）
           |
 go func() |-------------- 起動 -------------> srv.ListenAndServe()
           |                                     | リクエストを受け付け続ける
           v                                     |
     select で待つ                                |
     （起動の失敗か、止める合図か）                 |
```

## Q. チャネルとは？

**goroutine 同士で値を受け渡すための管** である。

```
   goroutine                                   serve
   errCh <- err   ======== errCh ========>     err := <-errCh
   （送る）           （管。値を1つ置ける）       （受け取る）
```

- `errCh <- 値`：管に値を送る
- `<-errCh`：管から値を受け取る。値が来るまで待つ
- `make(chan error, 1)`：`error` 型の値を通す管を作る。`1` は、受け取る人がいなくても1つ置いておける枠の数

goroutine は別の流れで動いているので、普通の `return` では結果を返せない。そこでチャネルを使って結果を届ける。

## Q. select とは？

**複数のチャネルを同時に待ち、先に値が届いたほうの処理を実行する** 文である。

```go
select {
case err := <-errCh:   // サーバーが止まった（起動の失敗など）
	return fmt.Errorf("serve: %w", err)
case <-ctx.Done():     // 止める合図が来た（Ctrl+C / SIGTERM）
}
```

## 2つの場面での動き

### 場面1：ポートがすでに使われていて、起動に失敗したとき

```
serve       go で起動 -- select で待つ ----------- <-errCh で受け取る -> エラーを返して終了
                |                                       ^
goroutine       +-> ListenAndServe() -> すぐ失敗 -> errCh <- err
```

### 場面2：Ctrl+C や SIGTERM で止めるとき

```
serve       go で起動 -- select で待つ -- ctx.Done() -- Shutdown() で待つ -------- 終了
                |                                          |
goroutine       +-> ListenAndServe() ... リクエストを処理 ... +-> ErrServerClosed が返る
                                                              -> errCh に置いて、goroutine も終わる
```

場面2では、`select` はすでに抜けているので、誰も `errCh` を読まない。

## Q. なぜ枠が1つ必要か（`make(chan error, 1)` の `1`）

チャネルの枠は、手渡しと郵便受けの違いに似ている。

| 作り方 | たとえ | 受け取る人がいないとき |
|---|---|---|
| `make(chan error)`（枠なし） | 手渡し。相手が来るまで、その場で待つ | 送る側の goroutine が永遠に待ち、消えずに残る（goroutine のリーク） |
| `make(chan error, 1)`（枠1つ） | 郵便受け。入れたら帰れる | 1つ置いて、goroutine はすぐ終われる |

場面2で誰も受け取らなくても、goroutine がきちんと終われるように、枠を1つ用意している。

## 動かして確かめる

```go
package main

import (
	"fmt"
	"time"
)

var start = time.Now()

func log(msg string) { fmt.Printf("%.1f秒  %s\n", time.Since(start).Seconds(), msg) }

func main() {
	log("main: 開始")

	ch := make(chan string, 1)
	go func() {
		log("goroutine: 時間のかかる仕事を始める（2秒）")
		time.Sleep(2 * time.Second)
		ch <- "仕事の結果"
	}()

	log("main: goroutine を起動した。待たずに次へ進む")

	select {
	case result := <-ch:
		log("main: チャネルから受け取った → " + result)
	case <-time.After(5 * time.Second):
		log("main: 5秒たったので、あきらめる")
	}
}
```

実行結果：

```
0.0秒  main: 開始
0.0秒  main: goroutine を起動した。待たずに次へ進む
0.0秒  goroutine: 時間のかかる仕事を始める（2秒）
2.0秒  main: チャネルから受け取った → 仕事の結果
```

- main は goroutine の終わりを待たずに、すぐ次の行に進んでいる（0.0秒）
- `select` で待ち、2秒後に届いた結果を受け取った
- `time.Sleep` を 6秒にすると、5秒のほうが先に来るので「あきらめる」が表示される

## JavaScript（フロントエンド）と比べると

| Go | JavaScript で近いもの |
|---|---|
| `go func() { ... }()` | `await` を付けずに async 関数を呼ぶ |
| チャネルで結果を受け取る | Promise の結果を受け取る |
| `select` | `Promise.race([...])`（先に終わったほうを使う） |
| `ctx.Done()` | `AbortSignal` の abort |

大きな違いもある。

- **JavaScript は1本の流れ（シングルスレッド）**。重い計算をすると、その間はほかの処理がすべて止まる。待ち時間の処理は、イベントループが順番に回している
- **Go の goroutine は、CPU の複数のコアで本当に同時に動く**。1つの goroutine が待ちに入っても、Go のランタイムがほかの goroutine に切り替える
- goroutine はとても軽く、数万個作っても問題ない。実は `net/http` のサーバーは、**リクエストごとに goroutine を1つ作って処理している**。たくさんのリクエストを同時にさばけるのはこのため
