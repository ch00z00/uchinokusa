-- name: CreateSuggestion :one
-- 世帯に同じ重複防止のキーの提案があれば（見送ったものも含む）、作らずに行を返さない（要件17-6）
INSERT INTO suggestions (
  household_id, plant_id, task_id, provider_id, kind, action, task_kind,
  task_interval_value, task_interval_unit, task_seasonal_intervals, proposed_date, condition_text,
  message_key, message_params, reason_key, reason_params, valid_from, valid_until, dedupe_key
) VALUES (
  sqlc.arg(household_id), sqlc.narg(plant_id), sqlc.narg(task_id), sqlc.arg(provider_id), sqlc.arg(kind), sqlc.arg(action), sqlc.narg(task_kind),
  sqlc.narg(task_interval_value), sqlc.narg(task_interval_unit), sqlc.arg(task_seasonal_intervals), sqlc.narg(proposed_date), sqlc.narg(condition_text),
  sqlc.arg(message_key), sqlc.arg(message_params), sqlc.arg(reason_key), sqlc.arg(reason_params), sqlc.arg(valid_from), sqlc.narg(valid_until), sqlc.arg(dedupe_key)
)
ON CONFLICT (household_id, dedupe_key) DO NOTHING
RETURNING *;

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
