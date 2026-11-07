-- name: ListHouseholdCalendars :many
-- 季節の初日を判定するための、世帯のタイムゾーンと半球
SELECT id, time_zone, hemisphere FROM households ORDER BY id;

-- name: LockSeasonalTasks :many
-- 季節ごとの周期を持つ有効なタスクをロックする。手で変えた予定日（due_overridden。スキップの確認日を含む）は計算し直さない（要件8-7）。
-- 最後の完了は、ロックを取ったあとに別の文で読む（FOR UPDATE は、待ったあとにロックした行しか読み直さないため）
SELECT * FROM care_tasks
WHERE household_id = $1 AND status = 'active' AND deleted_at IS NULL AND NOT due_overridden
  AND seasonal_intervals <> '{}'::jsonb
ORDER BY id
FOR UPDATE;

-- name: SetSeasonalNextDue :exec
UPDATE care_tasks SET next_due_on = $2 WHERE id = $1;

-- name: GetLastCompletionOn :one
-- 次回予定の起点にする、最後の完了の実施日。追加の記録は予定を変えないので除く（docs/decisions/0023）
SELECT performed_on FROM care_records
WHERE task_id = $1 AND outcome = 'completed' AND NOT is_extra
ORDER BY created_at DESC, id DESC
LIMIT 1;
