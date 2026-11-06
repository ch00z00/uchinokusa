-- name: GetCalendarPlant :one
-- カレンダーの権限・今日・季節を決めるため、植物の世帯・タイムゾーン・半球と、アーカイブしたかを返す
SELECT plants.household_id, households.time_zone, households.hemisphere, (plants.archived_at IS NOT NULL)::boolean AS archived
FROM plants
JOIN households ON households.id = plants.household_id
WHERE plants.id = $1;

-- name: ListMonthRecords :many
-- 実施日が月の範囲の記録（完了・スキップ）を、実施日・記録した順に返す（要件14-1）。タスクを削除した記録も含める
SELECT * FROM care_records
WHERE household_id = sqlc.arg(household_id) AND plant_id = sqlc.arg(plant_id)
  AND performed_on BETWEEN sqlc.arg(first_day)::date AND sqlc.arg(last_day)::date
ORDER BY performed_on, created_at, id;

-- name: ListPlantActiveTasks :many
-- 予定を展開する、植物の有効なタスク（一時停止・削除を除く）
SELECT * FROM care_tasks
WHERE household_id = $1 AND plant_id = $2 AND status = 'active' AND deleted_at IS NULL
ORDER BY kind, custom_name, id;

-- name: ListMonthNotes :many
-- 作成日時が [starts_at, ends_at) のメモを、古い順に返す（要件14-3）。月の境目は、世帯のタイムゾーンで呼び出し側が決める
SELECT * FROM notes
WHERE household_id = sqlc.arg(household_id) AND plant_id = sqlc.arg(plant_id)
  AND created_at >= sqlc.arg(starts_at)::timestamptz AND created_at < sqlc.arg(ends_at)::timestamptz
ORDER BY created_at, id;
