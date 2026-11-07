# 引き継ぎメモ

新しいエージェント（または新しいチャット）で作業を再開するときは、まずこのファイルを読むこと。

## 現在の状況（2026-11-07 時点）
- 仕様駆動開発（Kiro 方式）で、**要件・設計・タスクの3つのフェーズがすべて承認済み**
- 実装フェーズ中。**完了したタスク：1.1**（Go の API サーバーの雛形、ローカル開発環境）、**1.2**（API 定義、コードの生成、エラーの変換）、**1.3**（DB のスキーマ、migrate、sqlc、結合テストの基盤）、**2.1**（ID トークンの認証、`/v1/me`）、**2.2**（端末の登録・解除、`/v1/me/devices`）、**3.1**（次回予定の計算と季節の判定、`domain/schedule.go`）、**3.2**（植え替えの適期の判定、`domain/repotting.go`）、**4.1**（ロールにもとづく権限の判定、`domain/role.go`・`app/authorizer.go`）、**4.2**（世帯の作成・一覧・変更・削除、`/v1/households`）、**4.3**（メンバーの管理・退出・オーナー権限の移譲・通知設定）、**5.1**（招待の発行・一覧・取り消し）、**5.2**（招待の確認と参加、レート制限）、**6.1**（植物マスタ・初期データ・検索）、**6.2**（置き場所の登録・変更・削除）、**6.3**（植物の登録・一覧・編集・削除・アーカイブ）、**7.1**（お手入れタスクの作成・一覧・変更・削除、作成画面の初期値）、**7.2**（タスクの一時停止と再開）、**8.1**（完了の記録と、重複の防止）、**8.2**（スキップの記録）、**8.3**（取り消しと、まとめて完了）、**8.4**（記録の編集・削除と、履歴の取得）、**9.1**（今日やること一覧）、**9.2**（植物ごとのメモ）、**9.3**（植物ごとのカレンダー）、**10.1**（助言プロバイダの共通の仕組み、提案の一覧・採用・見送り）、**10.2**（植え替えのプロバイダ、植物の登録・タスクの変更とのつなぎ込み）、**11.1**（変更イベントの発行と、pg_notify での配信）、**11.2**（WebSocket での購読と配信、`/v1/ws`）、**12.1**（定期ジョブの実行基盤、FCM での送信）、**12.2**（完了通知・参加の通知の送信ジョブ）
  - サーバーの起動方法・環境変数は `server/README.md` を参照
- 仕様：`.kiro/specs/household-plant-care-mvp/`
  - `requirements.md`：要件（20項目、EARS の考え方で日本語化した受け入れ基準）
  - `design.md`：技術設計（構成図、設計判断、API、データモデル、テスト戦略）
  - `tasks.md`：実装タスク（大項目20、サブタスク47）
  - `spec.json`：フェーズと承認の状態
- ステアリング：`.kiro/steering/`（`product.md` / `tech.md` / `structure.md`）

## 次にやること
```
/kiro:spec-impl household-plant-care-mvp 12.3
```
- API 定義は `api/openapi.yaml`。変えたら `make gen-api`。アプリの型の生成は、`mobile/` を作るタスク 14.1 から有効になる（その際 `openapi-typescript` を devDependency にする）。生成し直し忘れの確認は `make check-gen`
- **タスクは1つずつ進める**（1.1 → 1.2 → 1.3 → 2.1 → 2.2 → 3.1 → 3.2 → 4.1 → 4.2 → 4.3 → 5.1 → 5.2 → 6.1 → 6.2 → 6.3 → 7.1 → 7.2 → 8.1 → 8.2 → 8.3 → 8.4 → 9.1 → 9.2 → 9.3 → 10.1 → 10.2 → 11.1 → 11.2 → 12.1 → 12.2 → 12.3 …）
- 世帯のデータを扱うユースケースは、最初に `Authorizer.Require(userID, householdID, action)` を通す（メンバーでなければ 404、ロール不足は 403）
- 世帯のデータを変える書き込みは、`postgres.withChanges` のトランザクションで行い、`ch.add(...)` で変更イベントを足す（コミットの直前にバージョンを進めて `pg_notify`）。書き込みを足したら、`TestChanges_EveryWrite` にも足す。操作した人は認証の middleware が `app.WithActor` で context に入れる。受け取りは `postgres.ChangeListener` で、`ws.Hub.Publish` に渡して、世帯を購読している WebSocket の接続に配る（docs/decisions/0029）
- WebSocket は `GET /v1/ws`（`Authorization` ヘッダーで認証）。接続したアプリは `subscribe {householdIds}` を送り、`ws.Hub` が `Authorizer.Require(..., read)` で所属を確かめて購読させる。メンバーの削除・ロールの変更のイベントで、本人の接続に `membership_revoked`・`role_changed` を送る。LISTEN をつなぎ直したとき（`ChangeListener.OnListen = hub.Reset`）と、送り待ちがあふれたときは、接続を閉じてつなぎ直させる。メッセージの取り決めは docs/decisions/0030
- 定期ジョブは `internal/job` の `Job`（`Name`・`Run`）を実装し、`cmd/uchinokusa/main.go` の `newJobs` で `job.NewRunner(...)` に登録する。`POST /internal/jobs/{名前}` で、Cloud Scheduler の OIDC トークン（`JOB_AUDIENCE`・`JOB_SERVICE_ACCOUNT`、ローカルでは未設定でエンドポイントなし）で呼ぶ。同じジョブは advisory lock で同時に1つ。続けて2回動いても結果が変わらないことは、各ジョブがユニーク制約・状態の更新で保証する。プッシュ通知は `app.PushService.Notify(ユーザーたち, domain.PushMessage)` で送り、FCM が UNREGISTERED と答えたトークンは消す（docs/decisions/0031）
- オーナーは常に1人（ロールの変更で owner は指定できず、交代は移譲だけ）。招待（5.x）でも owner のロールは発行しない
- `serve` には環境変数 `INVITE_BASE_URL` が必要（`.env.example` に仮の値あり）
- 通知のアウトボックス（`notification_outbox`）の完了・参加の通知は、ジョブ `notifications`（1分ごと）が「世帯・種類・操作した人」ごとに、最後の行から60秒（最初の行から最大10分）でまとめて送る。行のロック・送信・送信済みへの更新は1つのトランザクション（`OutboxRepository.Dispatch`）。文面は `domain.CompletionNotice`・`MemberJoinedNotice`（docs/decisions/0032）
- 植物マスタ（`server/db/seeds/species.yaml`）は34種。性質を出典つきで入れたのは10種で、残りの24種は名前だけ。性質を足すときは、値ごとに出典を確かめ、`sources` の `fields` に挙げる（`seed` とテストが、出典のない値を拒否する。docs/decisions/0015）
- 置き場所の ID だけのパス（`/locations/{id}`）は、置き場所から世帯を探してから `Authorizer.Require` を通す（`app/location.go` の `authorize`）。植物の ID だけのパス（`/plants/{id}`）も同じ形（`app/plant.go` の `authorize`）。タスクの ID だけのパスも同じ形にする
- アーカイブした植物（`plants.archived_at` が NULL でない）のタスクは、今日やること（9.1）・リマインダー（12.3）の対象から外す。植物の一覧の `nextDueOn` は、有効なタスクの `next_due_on` の最小値（docs/decisions/0017）
- タスクの予定日を手で変えると `care_tasks.due_overridden` が立つ。完了（8.1）で印を外す。スキップ（8.2）の確認日も印を立てる（docs/decisions/0021）。周期を変えても次回予定日は動かさない（docs/decisions/0018）
- 「今日」は世帯のタイムゾーンで決める（`domain.TodayIn`）。現在時刻は Service に関数（`now func() time.Time`）で渡し、テストでは固定する（`CareTaskService` を参照）。タスクの ID だけのパスでは `CareTaskRepository.TimeZoneOf` で世帯のタイムゾーンを取る
- 一時停止中のタスク（`care_tasks.status = 'paused'`）は、今日やること（9.1）・リマインダー（12.3）で `status = 'active'` に絞って外す。再開すると次回予定は `max(今の次回予定, 今日)`。一時停止中のタスクも完了でき、一時停止のままにする（docs/decisions/0019・0020）
- 完了・スキップは `CareRecordRepository` の1つのトランザクションで、`LockCareTask`（行ロック）→ domain の照合（`PlanCompletion`）→ 記録 → タスクの更新 → アウトボックス、の順に行う。スキップ（8.2）も同じ形で、照合は `CheckSkip`（今の回だけ）、アウトボックスには入れない。記録の `prev_next_due_on`・`prev_cycle_seq`・`prev_due_overridden` は取り消し（8.3）で戻す先。取り消しも同じ行ロックを取ってから最新の記録かを確かめる。記録の編集・削除も同じロックの中で行い、予定を決めた最新の完了のときだけ次回予定を計算し直す（docs/decisions/0020・0022・0023）
- メモの編集・削除は、`note.write` に加えて作成者本人かオーナーであることを `Membership.CanModifyNote` で確かめる。植物の ID だけのパスでは、`PlantLocator`（`PlantRepository.HouseholdOf`）で世帯を探す（docs/decisions/0025）
- カレンダーの予定は保存せず、`domain.ExpandPlans` で今の予定日から周期で展開する（2回目からは見込み。期限切れなら今日から数える）。予定の計算を変えるときは、完了の計算（`NextDueAfterCompletion`）とそろえる（docs/decisions/0026）
- 助言プロバイダは `internal/advisor` の `Advisor` を実装し、`cmd/uchinokusa/main.go` の `advisor.NewRegistry(...)` に登録する（10.2 で `advisor.Repotting{}` を登録）。提案を作るきっかけは `SuggestionService.Evaluate(ctx, app.AdviceEvent{...})` で、エラーを返さないので、植物の登録などの後にそのまま呼べる。`create_task` の提案は、作るタスクの周期（`TaskSchedule`）が必須（docs/decisions/0027）。植物の登録と、タスクの予定日に関わる変更（作成・予定日の変更・一時停止・再開・削除）のあとに評価する。予定日の変更で評価し直すと、プロバイダが返さなくなった前の提案は撤回（`expired`）される（docs/decisions/0028）
- DB の結合テストには Docker が必要（起動していないと testcontainers が失敗する）
- エンドポイントを追加するときは、`api/openapi.yaml` → `make gen-api` → `httpapi/server.go` にメソッドを実装、の順。認証は自動でかかり、ハンドラーでは `currentUser(ctx)` でユーザーを取れる
- DB のクエリは `server/db/queries/` に書き、`make gen-db` で生成する。結合テストは `pgtest.New(t)` でテストごとのデータベースを使う
- ユーザーはバックエンド（Go）の実装経験を積みたいので、**プロセスを一緒に確認しながら進めたい**と考えている
  - 各タスクの冒頭で「何を・なぜ作るか」を短く説明する
  - 実装後に、主要な判断（ライブラリの使い方、テストの意図など）を説明する
  - 1つのタスクが終わったら止まり、確認をもらってから次へ進む
- **バックエンドは TDD で進める**（2026-11-04 にユーザーと決定）：テストを先に書き、失敗（RED）を確かめてから実装する。主要な条件（ロック・世帯の条件など）は、実装を壊してテストが落ちることも確かめる
- タスクが終わったら、`tasks.md` のチェックボックスを `[x]` にし、コミットする
- push は大きいタスク（7, 8 など）の単位で行う。サブタスク（8.1 など）ごとにはコミットだけして、そのタスクのサブタスクがすべて終わったらまとめて push する（2026-11-05 にユーザーと決定）

## ユーザーとのやり取りで決まったこと（仕様に反映済み）
- 回答は日本語で行う
- MVPはコア機能＋共有機能。写真は扱わず、テキストのメモのみ。カレンダーは植物ごと
- 権限はオーナー／編集者／閲覧者。複数の世帯（自宅・実家・友人の家など）に所属できる
- 次回の予定は「実際に作業した日」を起点に計算する
- 「やるべきこと」と同じくらい「今日はやらない方がよいこと」を示すのが大事。条件つき（「もし〜なら〜」）の判断はユーザーに委ねる
- スキップしたタスクは未完了のまま、次の確認日（初期値は翌日）に再び表示する
- 通知はまとめて送る（多すぎる通知は避ける）。即時通知のニーズは、リリース後に検討する
- オフライン時の記録の一時保存は、今後のスプリントに回す
- 天気連動の助言はフェーズ2。ただし植物マスタと置き場所の項目は、MVPで用意する
- 技術：React Native（Expo）／Go＋PostgreSQL／Firebase Auth＋FCM／WebSocket（詳細は `.kiro/steering/tech.md`）

## 開発ログ
- リリース後に回す課題（直近で重要度の高くないもの）は `docs/backlog.md` に追加する。1st スプリントは MVP のリリースを優先する
- 設計や運用の判断は、背景と理由とあわせて `docs/decisions/` に1件1ファイルで残す（一覧は `docs/decisions/README.md`）
- 実装中に新しい判断（ライブラリの選定、構成の変更など）をしたら、記録を追加する
- ユーザーの技術的な質問と答えは、`docs/learning/` に話題ごとにまとめる（一覧は `docs/learning/README.md`）

## 環境
- リモート：https://github.com/ch00z00/uchinokusa.git（ブランチ `main`）
- ローカルのパス：`/Users/Yusuke1/choozoo/uchinokusa`
- 開発は VSCode 上の Claude Code で行う予定
