# OpenAPI（api/openapi.yaml）

- 日付：2026-10-26
- きっかけ：タスク 1.2 で作った `api/openapi.yaml`

## Q. 今の `api/openapi.yaml` には、エラーのスキーマしか定義されていない？

ほぼそう。コミット `42397d1`（タスク 1.2）の時点では、**エンドポイントは1つもなく**（`paths: {}`）、すべての API で共通に使う「土台」だけが定義されている。

| 部分 | 中身 |
|---|---|
| `info.description` | 表現のルール（キャメルケース、日付・日時の書き方、エラーの返し方）と、互換性のルール（追加だけを行う、知らない項目や値は無視する） |
| `servers` | `/v1`（URL の先頭にバージョンを付ける） |
| `paths` | 空。エンドポイントは、タスク 2.1 以降でそれぞれ追加する |
| `components.schemas` | `Date`、`DateTime`、`ErrorCode`、`Error` |
| `components.responses` | エラーのレスポンスの部品（400・401・403・404・409・422・500） |

### なぜ土台から作るのか

タスク 1.2 の目的は、エンドポイントを作ることではなく、**これから作るすべてのエンドポイントが従う決まりと、その仕組み** をそろえることである。

- エラーの形や日付の書き方を最初に決めておくと、あとのタスクでエンドポイントごとにばらばらにならない
- `components.responses` に部品を用意しておくと、エンドポイントを追加するときに `$ref: "#/components/responses/NotFound"` の1行でエラーのレスポンスを書ける
- 定義からコードを生成する流れ（`make gen-api` → `server/internal/adapter/httpapi/oapi/oapi.gen.go`）と、ドメインのエラーを HTTP のエラーに変える仕組み（`server/internal/adapter/httpapi/errors.go`）も、このタスクで用意した

フロントエンドで例えるなら、画面を作る前に、デザインシステム（色・文字・ボタンの部品）を先に作っておくようなもの。

### `/healthz` が定義にないのはなぜか

`/healthz` は、アプリが使う API ではなく、Cloud Run などの監視の仕組みが使う運用のためのエンドポイントだから。`/v1` の外にあり、アプリとの取り決め（OpenAPI）には含めていない。

## Q. `server/internal/adapter/httpapi/oapi/` の中身は？

**`api/openapi.yaml` から、Go のコードを自動で作る（コード生成する）場所** である。生成には oapi-codegen というツールを使う。ファイルは3つある。

| ファイル | 役割 | 手で編集する？ |
|---|---|---|
| `config.yaml` | oapi-codegen の設定（何を生成するか） | する |
| `generate.go` | 生成のコマンドを書いておく場所 | する |
| `oapi.gen.go` | 生成されたコード | **しない**（先頭に `DO NOT EDIT` とある） |

フロントエンドで例えるなら、GraphQL Code Generator や openapi-typescript で、スキーマから TypeScript の型を作るのと同じ。

### 生成の流れ

```
api/openapi.yaml ──(make gen-api)──> go generate ──> oapi-codegen ──> oapi.gen.go
                                          ^                ^
                                   generate.go の        config.yaml の
                                   //go:generate の行     設定に従う
```

- `generate.go` の `//go:generate go tool oapi-codegen ...` は、「`go generate` を実行したら、このコマンドを実行する」という印
- `go tool oapi-codegen` は、`go.mod` の `tool` の行で決めたバージョンの oapi-codegen を実行する（Go 1.24 からの機能）。npm の devDependencies に入れたツールを `npx` で実行するのに近く、開発者ごとにバージョンがずれない

### `config.yaml` の設定

| 設定 | 意味 |
|---|---|
| `models: true` | スキーマから Go の型を作る |
| `std-http-server: true` | 標準の `net/http` で動くルーティングを作る（Echo や Gin などのフレームワークを使わない） |
| `strict-server: true` | 型つきのリクエストを受け取り、型つきのレスポンスを返すインターフェースを作る（下で説明） |
| `always-prefix-enum-values: true` | 定数名に型名を付ける（`NotFound` ではなく `ErrorCodeNotFound`）。列挙値が増えても名前がぶつからない |
| `skip-prune: true` | エンドポイントから使われていないスキーマも生成する。今はエンドポイントがないので、これがないと `Error` が生成されない |

### `oapi.gen.go` に生成されているもの

**1. 型**

```go
type ErrorCode string
const (
	ErrorCodeNotFound ErrorCode = "not_found"
	...
)

type Error struct {
	Code    ErrorCode          `json:"code"`
	Fields  *map[string]string `json:"fields,omitempty"`  // 省略できる項目はポインタになる
	Message string             `json:"message"`
}

type Date = openapi_types.Date
type DateTime = time.Time
```

- `` `json:"code"` `` は構造体タグ。JSON にするときの項目名を指定している（Go の項目名は大文字の `Code`、JSON はキャメルケースの `code`）
- 定義の `required` に入っていない `fields` はポインタ（`*`）になり、`nil` なら JSON から省かれる（`omitempty`）

**2. サーバーのインターフェース（今は空）**

```go
type StrictServerInterface interface {
}
```

エンドポイントを定義に追加すると、ここにメソッドが増える。たとえば世帯の一覧を返すエンドポイントを追加すると、次のようなメソッドが生成される（名前は例）。

```go
ListHouseholds(ctx context.Context, request ListHouseholdsRequestObject) (ListHouseholdsResponseObject, error)
```

**3. ルーティングのつなぎ込み**

`HandlerWithOptions` などの関数。定義にある URL とメソッドを `http.ServeMux` に登録し、URL のパラメーターを読み取る。

### strict-server とは

普通の `net/http` のハンドラーは、`r.Body` から JSON を読み、`w` に JSON を書く処理を、毎回自分で書く。strict-server では、その部分を生成されたコードが受け持つ。自分で書くのは「型つきのリクエストを受け取り、型つきのレスポンスを返す」ことだけになる。

```
リクエスト（例：GET /v1/households）
   |
   v
http.ServeMux
   |
   v
HandlerWithOptions が登録したルート     <- 生成：URL のパラメーターを読み取る
   |                                    読めなければ onRequestError -> 400
   v
strictHandler                          <- 生成：JSON を Go の型に変える
   |
   v
server のメソッド                       <- 自分で書く（httpapi/server.go）
   |                                    エラーを返したら writeError -> 種類に応じたステータス
   v
strictHandler が Go の型を JSON にして返す
```

この組み立ては `httpapi/router.go` の `NewRouter` で行っている。エラーの扱いを `writeError`（`httpapi/errors.go`）の1か所に集めているので、どのエンドポイントでもエラーの形がそろう。

### 書き忘れをコンパイルで見つける仕組み

`httpapi/server.go` には、次の1行がある。

```go
var _ oapi.StrictServerInterface = (*server)(nil)
```

「`server` は `StrictServerInterface` を満たしているはず」とコンパイラーに確かめさせる書き方である（`_` は値を使わないという意味）。定義にエンドポイントを追加して生成し直すと、インターフェースにメソッドが増える。`server` にそのメソッドを書かないと、**コンパイルエラーになる**。定義と実装のずれを、実行する前に見つけられる。

### なぜ `adapter/httpapi` の下にあるか

生成されたコードは、HTTP と JSON のための型である。植物や予定の計算などのドメインのロジックは、これを知らなくてよい。HTTP の層（`adapter/httpapi`）に閉じ込めておくと、API の形が変わっても、ドメインのコードに影響しない。
