# 0003. API のコード生成は oapi-codegen の strict-server を使い、エラーの変換を一か所にまとめる

- 状態：採用
- 日付：2026-10-26
- 関連：タスク 1.2、`api/openapi.yaml`、`server/internal/adapter/httpapi/errors.go`、要件19-9・1-3・4-3・4-4

## 背景
`api/openapi.yaml` から Go のサーバーのコードを生成する（0001）。oapi-codegen には生成の方式がいくつかあり、どれを使うかで、ハンドラーの書き方とエラーの扱い方が変わる。
あわせて、ドメインのエラーを HTTP のステータスとエラーコードに変換する場所を決める必要があった。

## 検討した案
| 案 | 内容 | 評価 |
|---|---|---|
| A | `std-http-server` のみ | ハンドラーは `func(w, r)` の形。レスポンスを自分で書くため、定義と違う形を返してもコンパイルで気づけない |
| B | `std-http-server` + `strict-server` | 採用。ハンドラーは「型つきのリクエストを受け取り、型つきのレスポンスかエラーを返す」形になる |
| C | gin / echo 用の生成 | 標準の `net/http` で学ぶ方針（tech.md）に合わない |

## 決定
1. oapi-codegen の **`std-http-server` + `strict-server`** で生成する。生成先は `server/internal/adapter/httpapi/oapi/`（`go generate` で生成し、生成したファイルもコミットする）
2. oapi-codegen は Go 1.24 以降の **`tool` ディレクティブ**（`go.mod`）で管理し、`go tool oapi-codegen` で実行する。バージョンを `go.mod` で固定でき、別途インストールが要らない
3. ドメインのエラー（`domain.ErrUnauthenticated` / `ErrForbidden` / `ErrNotFound` / `ConflictError` / `ValidationError`）から HTTP への変換は、**`httpapi.writeError` の一か所**で行う。strict-server の「リクエストを読めなかったとき」「ハンドラーがエラーを返したとき」の両方のフックを、ここにつなぐ
4. エラーの `code` は、API 定義の **列挙（`ErrorCode`）** として定義する。サーバーは生成された定数を使い、定義にない衝突のコードは汎用の `conflict` にして返す（アプリが分岐できない値を返さない）
5. 入力の形式の誤り（JSON として読めない、など）は **400 `bad_request`**、内容の誤り（必須の項目がない、長すぎる、など）は **422 `validation_failed`** と分ける

## 理由
- strict-server なら、各エンドポイントのハンドラーは「成功のレスポンスを返す」か「エラーを返す」だけになり、エラーの書き方がエンドポイントごとにばらつかない
- エラーの変換を一か所にすると、「所属していない世帯は 404 にする」（要件4-4）のような規約を、全エンドポイントで確実に守れる
- 500 の原因（DB の接続先など）はログにだけ残し、レスポンスには出さない

## トレードオフ
- strict-server は、レスポンスの型が多くなり、生成されるコードが大きくなる
- エンドポイントがまだないため、参照されないスキーマ（`Error` など）も生成されるように `skip-prune: true` を設定している

## 見直す条件
- oapi-codegen が OpenAPI 3.1 の機能（`type: [string, "null"]` など）に対応しきれず、定義の書き方が制限されるとき
