-- name: ListReminderCandidates :many
-- リマインダーが有効で、端末があるメンバー。タイムゾーンは、メンバーが最後に使った端末のものにする（要件15-7）
SELECT m.id AS membership_id, m.household_id, m.user_id, m.reminder_time, d.time_zone
FROM memberships m
JOIN LATERAL (
  SELECT time_zone FROM user_devices
  WHERE user_id = m.user_id
  ORDER BY last_seen_at DESC, id DESC
  LIMIT 1
) d ON true
WHERE m.reminder_enabled
ORDER BY m.id;

-- name: ListReminderTasks :many
-- その日以前の未完了タスク。今日やること一覧（ListDueTasks）と同じ条件（有効・削除していない・アーカイブしていない植物）
SELECT care_tasks.kind, care_tasks.custom_name
FROM care_tasks
JOIN plants ON plants.id = care_tasks.plant_id
WHERE care_tasks.household_id = $1 AND care_tasks.status = 'active' AND care_tasks.deleted_at IS NULL
  AND care_tasks.next_due_on <= sqlc.arg(local_date)::date AND plants.archived_at IS NULL
ORDER BY care_tasks.next_due_on, plants.name, care_tasks.kind, care_tasks.id;

-- name: CreateReminderDispatch :one
-- その日の送信を記録する。ユニーク制約で1日1回にする（要件15-2）。すでに送っていれば、行を返さない。
-- 対象を選んだあとでリマインダーを切った・世帯を抜けたメンバーも、行を返さない
INSERT INTO reminder_dispatches (membership_id, local_date)
SELECT memberships.id, sqlc.arg(local_date)::date FROM memberships
WHERE memberships.id = sqlc.arg(membership_id) AND memberships.reminder_enabled
ON CONFLICT (membership_id, local_date) DO NOTHING
RETURNING reminder_dispatches.id;
