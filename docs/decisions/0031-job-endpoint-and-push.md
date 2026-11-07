# 0031. ジョブは Cloud Scheduler の OIDC トークンで呼び、同じジョブを advisory lock で同時に1つにする

- 状態：採用
- 日付：2026-11-07
- 関連：タスク 12.1、`server/internal/job/`、`server/internal/adapter/gcp/`、`server/internal/adapter/firebase/messenger.go`、`server/internal/app/push.go`、`server/internal/adapter/httpapi/jobs.go`、要件15・16
- 前提：設計の「ジョブの共通の取り決め」「ReminderJob」の無効なトークン

## 決定
1. **ジョブは `POST /internal/jobs/{name}` で、Cloud Scheduler のサービスアカウントの OIDC トークン（Google の ID トークン）だけを受け付ける**
   - API は、アプリから呼ぶので、Cloud Run の IAM で閉じられない（だれでも届く）。そのため、トークンをサーバーで確かめる
   - 確かめること：署名（Google の公開鍵）・宛先（`aud` が `JOB_AUDIENCE`）・期限・発行者（`iss` が Google）・呼び出したアカウント（`email` が `JOB_SERVICE_ACCOUNT` で、`email_verified`）
   - `google.golang.org/api/idtoken` の `Validate` は、発行者を確かめないので、自分で確かめる。また Google のどのアカウントでも正しいトークンは作れるので、アカウントの確認が欠かせない
   - トークンとして正しくなければ 401、ほかのアカウントなら 403。アプリのユーザーの ID トークン（Firebase）では呼べない
   - `/v1` の外に置き、OpenAPI には入れない（アプリの API ではないため）
   - `JOB_AUDIENCE`・`JOB_SERVICE_ACCOUNT` の両方がそろったときだけ登録する。片方だけなら起動しない（設定の誤り）。ローカルでは両方とも未設定にし、登録しない
   - Cloud Scheduler のジョブは、対象の URL を `<JOB_AUDIENCE>/internal/jobs/<名前>`、OIDC の対象（audience）を `JOB_AUDIENCE`、サービスアカウントを `JOB_SERVICE_ACCOUNT` にする
2. **同じジョブは、どのインスタンスでも同時に1つだけ動かす（PostgreSQL の advisory lock）**
   - Cloud Scheduler の再試行や、前の実行が長引いたときに、同じジョブが重なって動かないようにする。ほかで実行中なら、実行せずに `skipped` を返す（200）
   - ロックはデータベースの接続が持つ（`pg_try_advisory_lock`）。プロセスが落ちても、接続が切れれば外れる。外せなかった接続はプールに戻さず閉じる
   - これは「重ならない」ことの保証で、「続けて2回動いても結果が変わらない」ことは、各ジョブがユニーク制約と状態の更新で保証する（12.2・12.3）
   - 新しいテーブルは増やさない（ロックの状態を自分で持たない）
3. **実行の結果を、ジョブごとに1行のログ（`job finished` / `job failed`）とレスポンスに出す**
   - 項目：ジョブ名・処理件数（`processed`）・失敗件数（`failed`）・`skipped`・所要時間（`duration_ms`）。ログベースのメトリクス（設計の監視）に使う
   - ジョブが失敗したら 500 を返し、Cloud Scheduler の再試行に任せる
4. **プッシュ通知は `app.PushService.Notify(ユーザーたち, 通知)` で送り、FCM が `UNREGISTERED` と答えたトークンの端末を消す**
   - FCM には、1回 500 件までに分けて送る（`SendEachForMulticast`）。結果はトークンごとに「届いた・無効・失敗」に分ける
   - `INVALID_ARGUMENT` では消さない。トークンではなく通知の中身の誤り（こちらの不具合）でも返り、消すとすべての端末を失いかねないため
   - トークンを消せなくても、送った結果は返す（次に送るときに、また無効と答えるので、そのときに消す）
   - FCM に届かなければエラーを返し、ジョブが次の実行でやり直す

## 残る課題
- 12.1 の時点では、登録したジョブはない。プッシュ通知の送信（`PushService`）も、ジョブで使うタスク 12.2 から `main.go` でつなぐ
- ローカルでジョブを試す方法（コマンドで直接実行するなど）は、12.2 で必要になったら用意する
