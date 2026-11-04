-- name: GetTaskPlant :one
-- タスクを作る植物の世帯と、世帯のタイムゾーン、植物マスタのお手入れの目安（紐づかなければ NULL）
SELECT plants.household_id, households.time_zone, species.recommended_care
FROM plants
JOIN households ON households.id = plants.household_id
LEFT JOIN species ON species.id = plants.species_id
WHERE plants.id = $1;

-- name: ListCareTasks :many
-- 削除していないタスク（一時停止中を含む）を、作成の古い順に返す
SELECT * FROM care_tasks
WHERE household_id = $1 AND plant_id = $2 AND deleted_at IS NULL
ORDER BY created_at, id;

-- name: CreateCareTask :one
-- 植物は (plant_id, household_id) の外部キーで、同じ世帯のものに限られる
INSERT INTO care_tasks (household_id, plant_id, kind, custom_name, interval_value, interval_unit, seasonal_intervals, condition_text, note, next_due_on)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetCareTaskHouseholdID :one
-- タスクの ID だけで呼ばれる API（/tasks/{id}）で、権限を確かめる世帯を探す。削除したタスクは見つからないものとする
SELECT household_id FROM care_tasks WHERE id = $1 AND deleted_at IS NULL;

-- name: GetCareTask :one
SELECT * FROM care_tasks WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;

-- name: UpdateCareTask :one
-- custom_name・周期・季節ごとの周期は NULL なら変えない。実行条件・メモは set_* が true のときだけ変える（NULL でなくす）
-- 予定日を変えたら、手で変えた印（due_overridden）を立てる。季節の切り替えで計算し直す対象から外すため
UPDATE care_tasks SET
  custom_name        = COALESCE(sqlc.narg(custom_name)::text, custom_name),
  interval_value     = COALESCE(sqlc.narg(interval_value)::integer, interval_value),
  interval_unit      = COALESCE(sqlc.narg(interval_unit)::text, interval_unit),
  seasonal_intervals = COALESCE(sqlc.narg(seasonal_intervals)::jsonb, seasonal_intervals),
  condition_text     = CASE WHEN sqlc.arg(set_condition_text)::boolean THEN sqlc.narg(condition_text)::text ELSE condition_text END,
  note               = CASE WHEN sqlc.arg(set_note)::boolean THEN sqlc.narg(note)::text ELSE note END,
  next_due_on        = COALESCE(sqlc.narg(next_due_on)::date, next_due_on),
  due_overridden     = due_overridden OR sqlc.narg(next_due_on)::date IS NOT NULL
WHERE id = sqlc.arg(id) AND household_id = sqlc.arg(household_id) AND deleted_at IS NULL
RETURNING *;

-- name: DeleteCareTask :execrows
-- 論理削除にして、記録（care_records.task_id）を履歴として残す（要件8-10）
UPDATE care_tasks SET deleted_at = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;
