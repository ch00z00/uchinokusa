# db/queries と sqlc

- 日付：2026-10-27
- きっかけ：タスク 1.3（コミット `6892060`）で、`server/db/` の下に `migrations/` と `queries/` の2つができた理由

## Q. `db/migrations/` と `db/queries/` は、何が違う？

どちらも SQL ファイルだが、書いてある SQL の種類と、使われ方がまったく違う。

| | `db/migrations/` | `db/queries/` |
|---|---|---|
| 書いてある SQL | テーブルを作る・変える（`CREATE TABLE`、`ALTER TABLE`） | アプリがデータを読み書きする（`SELECT`、`INSERT`、`UPDATE`、`DELETE`） |
| 答える問い | DB に **どんな入れ物があるか** | アプリが DB に **何を頼むか** |
| いつ実行されるか | `migrate` のときに、DB ごとに1回だけ | サーバーが動いている間、リクエストのたびに |
| 読むツール | goose（適用する）と sqlc（列の型を知るため） | sqlc（Go のコードを生成する） |
| ファイルの扱い | 追加だけの履歴。適用済みのファイルは書き換えない | 今の形が正。いつでも自由に書き換えてよい |

扱いのルールが正反対なので、ディレクトリを分けている。

## Q. `queries/` は何のため？

**sqlc というツールに渡して、型つきの Go のコードを生成するための SQL** を置く場所である。ファイルの SQL を、そのまま DB で実行するわけではない。

```
db/migrations/*.sql ----+  （列の名前と型を知るため）
                        +--> sqlc ---> internal/adapter/postgres/sqlcgen/*.go
db/queries/*.sql -------+  （どんな関数を作るかを知るため）     （生成されたコード。DO NOT EDIT）
```

生成は `make gen-db`（中身は `go generate` → `go tool sqlc generate`）で行う。設定は `server/sqlc.yaml`。

### 例：`db/queries/users.sql`

```sql
-- name: CreateUser :one
INSERT INTO users (firebase_uid, display_name, auth_provider)
VALUES ($1, $2, $3)
RETURNING *;
```

- `-- name: CreateUser` は、生成する Go の関数の名前
- `:one` は、結果の行が1つであること。ほかに `:many`（複数の行をスライスで返す）、`:exec`（結果を返さない）などがある
- `$1`、`$2`、`$3` は、あとから値を入れる場所（プレースホルダー）

ここから、次の Go のコードが生成される（`sqlcgen/users.sql.go`）。

```go
type CreateUserParams struct {
	FirebaseUID  string
	DisplayName  string
	AuthProvider string
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
```

`users` テーブルの形も、Go の構造体になる（`sqlcgen/models.go`）。

```go
type User struct {
	ID           uuid.UUID
	FirebaseUID  string
	DisplayName  string
	AuthProvider string
	DeletedAt    *time.Time   // NULL になりうる列は、ポインターになる
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

アプリ側（今はテスト `queries_test.go`）では、次のように呼ぶだけでよい。

```go
created, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
	FirebaseUID: "uid-1", DisplayName: "...", AuthProvider: "google.com",
})
```

### sqlc で何がうれしいか

sqlc がないと、Go で DB を使うには、SQL を文字列で書き、結果の列を1つずつ変数に読み込む（`row.Scan(&id, &name, ...)`）コードを、毎回手で書く。

| 手で書く場合の問題 | sqlc での解決 |
|---|---|
| SQL の列名を書き間違えても、実行するまで気づかない | 生成のときに、マイグレーションと照らし合わせて誤りを見つける |
| `Scan` の順番や型を間違えやすい | `Scan` のコードを生成するので、間違えない |
| テーブルに列を足すと、関係する `Scan` をすべて直す必要がある | 生成し直すだけ |
| 引数の数や型の誤り | 引数が構造体（`CreateUserParams`）になり、コンパイルで見つかる |

たとえば `SELECT *` と書いても、生成されたコードでは `SELECT id, firebase_uid, ...` と列が展開されている。生成した時点のテーブルの形に固定されるので、あとで列が増えても、コードがずれない。

### OpenAPI のコード生成と同じ考え方

| | API（タスク 1.2） | DB（タスク 1.3） |
|---|---|---|
| 形の定義 | `api/openapi.yaml` | `db/migrations/*.sql` |
| やりたいこと | （エンドポイントは定義の中に書く） | `db/queries/*.sql` |
| ツール | oapi-codegen | sqlc |
| 生成先 | `httpapi/oapi/oapi.gen.go` | `postgres/sqlcgen/*.go` |

どちらも「取り決めを先に書き、型つきの Go のコードは生成する」という方針である。

### フロントエンドで例えるなら

GraphQL Code Generator にいちばん近い。

| GraphQL | sqlc |
|---|---|
| `schema.graphql`（どんな型があるか） | `db/migrations/*.sql`（どんなテーブルがあるか） |
| `*.graphql` の query / mutation（何を取りに行くか） | `db/queries/*.sql`（何を読み書きするか） |
| 生成された型つきの hooks | 生成された型つきの Go の関数 |

Prisma のような ORM との違いは、**SQL を自分で書く** こと。ORM は SQL を隠して、メソッドから SQL を作る。sqlc は、自分で書いた SQL をそのまま使い、型の部分だけを生成する。どんな SQL が実行されるかがはっきり見えるので、性能の調整やインデックスの検討がしやすい。

### このプロジェクトでの今の状態

`db/queries/` には、今は `users.sql`（`CreateUser` と `GetUserByFirebaseUID` の2つ）しかない。仕組みが動くことを確かめるための最小限で、タスクごとに、必要な SQL をここに追加していく（例：世帯なら `households.sql`）。

`db/db.go` で実行ファイルに埋め込んでいるのは `migrations/*.sql` だけである。`queries/` の SQL は、生成された Go のコードの中に文字列として入るので、ファイルとして持ち運ぶ必要がない。
