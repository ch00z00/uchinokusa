-- name: ListDueTasks :many
-- 今日やること一覧（要件9-1）：予定日が今日以前の、有効なタスク（一時停止・削除を除く）で、アーカイブしていない植物のもの
-- 期限切れの古いものから並べる。care_tasks (household_id, status, next_due_on) のインデックスを使う
SELECT sqlc.embed(care_tasks), plants.name AS plant_name, plants.location_id, locations.name AS location_name
FROM care_tasks
JOIN plants ON plants.id = care_tasks.plant_id
LEFT JOIN locations ON locations.id = plants.location_id
WHERE care_tasks.household_id = $1 AND care_tasks.status = 'active' AND care_tasks.deleted_at IS NULL
  AND care_tasks.next_due_on <= sqlc.arg(today)::date AND plants.archived_at IS NULL
ORDER BY care_tasks.next_due_on, plants.name, care_tasks.kind, care_tasks.id;

-- name: GetNextTask :one
-- 今日やることがないときの、直近の次の予定（要件9-6）。対象のタスクは ListDueTasks と同じ
SELECT sqlc.embed(care_tasks), plants.name AS plant_name, plants.location_id, locations.name AS location_name
FROM care_tasks
JOIN plants ON plants.id = care_tasks.plant_id
LEFT JOIN locations ON locations.id = plants.location_id
WHERE care_tasks.household_id = $1 AND care_tasks.status = 'active' AND care_tasks.deleted_at IS NULL
  AND care_tasks.next_due_on > sqlc.arg(today)::date AND plants.archived_at IS NULL
ORDER BY care_tasks.next_due_on, plants.name, care_tasks.kind, care_tasks.id
LIMIT 1;

-- name: ListActiveTaskSuggestions :many
-- タスクへの、今日の時点で有効な「やらない方がよい」「条件つき」の提案（要件9-4）。採用・見送り・期限切れは除く
SELECT * FROM suggestions
WHERE household_id = $1 AND task_id = ANY(sqlc.arg(task_ids)::uuid[])
  AND status = 'pending' AND kind IN ('avoid', 'conditional')
  AND valid_from <= sqlc.arg(today)::date AND (valid_until IS NULL OR valid_until >= sqlc.arg(today)::date)
ORDER BY created_at, id;
