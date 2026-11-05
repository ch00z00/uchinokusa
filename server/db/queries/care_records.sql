-- name: LockCareTask :one
-- 完了・スキップの前に、タスクの行をロックする（設計の判断2）。同じタスクへの同時の完了は、先のコミットを待ってから予定の回を照らし合わせる
-- 次回予定の計算に使う、世帯の半球もあわせて返す
SELECT sqlc.embed(care_tasks), households.hemisphere
FROM care_tasks
JOIN households ON households.id = care_tasks.household_id
WHERE care_tasks.id = $1 AND care_tasks.household_id = $2 AND care_tasks.deleted_at IS NULL
FOR UPDATE OF care_tasks;

-- name: CreateCompletionRecord :one
-- 作業の種類と作業した人の名前は、記録の時点の値を写して持つ（タスクの削除・メンバーの退出のあとも表示するため）
-- prev_* は、取り消し（8.3）で戻す先。追加の記録はタスクを変えないので NULL にする
INSERT INTO care_records (
  household_id, plant_id, task_id, task_kind, task_custom_name, outcome, performed_on,
  actor_user_id, actor_display_name, cycle_seq, is_extra, prev_next_due_on, prev_cycle_seq
)
SELECT sqlc.arg(household_id), sqlc.arg(plant_id), sqlc.arg(task_id), sqlc.arg(task_kind), sqlc.narg(task_custom_name), 'completed', sqlc.arg(performed_on),
  users.id, users.display_name, sqlc.arg(cycle_seq), sqlc.arg(is_extra), sqlc.narg(prev_next_due_on), sqlc.narg(prev_cycle_seq)
FROM users
WHERE users.id = sqlc.arg(actor_user_id)
RETURNING *;

-- name: AdvanceCareTask :one
-- 完了で、次回予定と予定の回を進める。次回予定は周期から計算し直したので、手で変えた印（due_overridden）を外す
UPDATE care_tasks SET next_due_on = $2, cycle_seq = $3, due_overridden = false
WHERE id = $1
RETURNING *;

-- name: GetCycleCompletion :one
-- 予定の回の完了の記録（追加の記録を除く。部分ユニーク制約で1件まで）
SELECT * FROM care_records
WHERE task_id = $1 AND cycle_seq = $2 AND outcome = 'completed' AND NOT is_extra;

-- name: EnqueueCompletion :exec
-- 完了の通知を予約する。送信とまとめ（要件16-4）は、ジョブ（タスク 12.2）が行う
INSERT INTO notification_outbox (household_id, kind, actor_user_id, record_id)
VALUES ($1, 'completion', $2, $3);

-- name: CreateSkipRecord :one
-- スキップは作業をしたことにしない（outcome = skipped）。理由は、助言の改善に使えるよう、決まった値と自由入力に分けて持つ（要件19-7）
-- prev_* は、取り消し（8.3）で戻す先
INSERT INTO care_records (
  household_id, plant_id, task_id, task_kind, task_custom_name, outcome, performed_on,
  actor_user_id, actor_display_name, skip_reason_code, skip_reason_text, recheck_on, cycle_seq, prev_next_due_on, prev_cycle_seq
)
SELECT sqlc.arg(household_id), sqlc.arg(plant_id), sqlc.arg(task_id), sqlc.arg(task_kind), sqlc.narg(task_custom_name), 'skipped', sqlc.arg(performed_on),
  users.id, users.display_name, sqlc.narg(skip_reason_code), sqlc.narg(skip_reason_text), sqlc.arg(recheck_on), sqlc.arg(cycle_seq),
  sqlc.arg(prev_next_due_on), sqlc.arg(prev_cycle_seq)
FROM users
WHERE users.id = sqlc.arg(actor_user_id)
RETURNING *;

-- name: DeferCareTask :one
-- スキップで、次回予定を確認日にする。予定の回は進めない（スキップした回に、あとから完了できる）
-- 確認日はユーザーが決めた日なので、手で変えた印（due_overridden）を立て、季節の切り替えで動かさない。次の完了で外れる
UPDATE care_tasks SET next_due_on = $2, due_overridden = true
WHERE id = $1
RETURNING *;
