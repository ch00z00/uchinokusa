-- name: CreateSuggestion :one
-- 世帯に同じ重複防止のキーの提案があれば（見送ったものも含む）、作らずに行を返さない（要件17-6）
-- ただし、撤回した（expired の）提案なら、新しい内容で未回答に戻して返す（予定日を戻したときに、助言を出し直すため）
INSERT INTO suggestions (
  household_id, plant_id, task_id, provider_id, kind, action, task_kind,
  task_interval_value, task_interval_unit, task_seasonal_intervals, proposed_date, condition_text,
  message_key, message_params, reason_key, reason_params, valid_from, valid_until, dedupe_key
) VALUES (
  sqlc.arg(household_id), sqlc.narg(plant_id), sqlc.narg(task_id), sqlc.arg(provider_id), sqlc.arg(kind), sqlc.arg(action), sqlc.narg(task_kind),
  sqlc.narg(task_interval_value), sqlc.narg(task_interval_unit), sqlc.arg(task_seasonal_intervals), sqlc.narg(proposed_date), sqlc.narg(condition_text),
  sqlc.arg(message_key), sqlc.arg(message_params), sqlc.arg(reason_key), sqlc.arg(reason_params), sqlc.arg(valid_from), sqlc.narg(valid_until), sqlc.arg(dedupe_key)
)
ON CONFLICT (household_id, dedupe_key) DO UPDATE SET
  plant_id = EXCLUDED.plant_id, task_id = EXCLUDED.task_id, provider_id = EXCLUDED.provider_id, kind = EXCLUDED.kind, action = EXCLUDED.action,
  task_kind = EXCLUDED.task_kind, task_interval_value = EXCLUDED.task_interval_value, task_interval_unit = EXCLUDED.task_interval_unit,
  task_seasonal_intervals = EXCLUDED.task_seasonal_intervals, proposed_date = EXCLUDED.proposed_date, condition_text = EXCLUDED.condition_text,
  message_key = EXCLUDED.message_key, message_params = EXCLUDED.message_params, reason_key = EXCLUDED.reason_key, reason_params = EXCLUDED.reason_params,
  valid_from = EXCLUDED.valid_from, valid_until = EXCLUDED.valid_until,
  status = 'pending', responded_by = NULL, responded_at = NULL, created_at = now()
WHERE suggestions.status = 'expired'
RETURNING *;

-- name: RetractSuggestions :many
-- プロバイダの、タスクへの未回答の提案のうち、今回返さなかったもの（キーが keep_keys にないもの）を撤回する
UPDATE suggestions SET status = 'expired'
WHERE household_id = sqlc.arg(household_id) AND provider_id = sqlc.arg(provider_id) AND task_id = sqlc.arg(task_id)
  AND status = 'pending' AND dedupe_key <> ALL(sqlc.arg(keep_keys)::text[])
RETURNING id, plant_id;

-- name: GetSuggestionHouseholdID :one
-- 権限を確かめるため、提案が属する世帯を探す
SELECT household_id FROM suggestions WHERE id = $1;

-- name: LockSuggestion :one
-- 採用・見送りの前に、提案の行をロックする。同時の採用で、タスクを二重に作らないため
SELECT * FROM suggestions WHERE id = $1 AND household_id = $2 FOR UPDATE;

-- name: RespondSuggestion :one
-- ユーザーの対応（採用・見送り）と、対応した人・日時を記録する（要件19-6）
UPDATE suggestions SET status = $2, responded_by = $3, responded_at = now()
WHERE id = $1
RETURNING *;

-- name: ListActiveSuggestions :many
-- その日に有効な（未回答で、有効期間の中の）提案を、作った順に返す
SELECT * FROM suggestions
WHERE household_id = $1 AND status = 'pending'
  AND valid_from <= sqlc.arg(on_date)::date AND (valid_until IS NULL OR valid_until >= sqlc.arg(on_date)::date)
ORDER BY created_at, id;

-- name: GetAdviceSpecies :one
-- プロバイダに渡す、植物マスタの種類
SELECT * FROM species WHERE id = $1;

-- name: GetAdviceLocation :one
-- プロバイダに渡す、植物の置き場所
SELECT * FROM locations WHERE id = $1 AND household_id = $2;
