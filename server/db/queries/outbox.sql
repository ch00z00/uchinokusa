-- name: ReadyNoticeGroups :many
-- 送ってよいまとまり（世帯・種類・操作した人）。最後の行から一定時間がたった（続けて完了していない）か、
-- 最初の行から長く待った（続けて完了していても送る）まとまりを、待っている時間の長い順に返す（要件16-4）
SELECT household_id, kind, actor_user_id
FROM notification_outbox
WHERE status = 'pending'
GROUP BY household_id, kind, actor_user_id
HAVING max(created_at) <= sqlc.arg(quiet_before)::timestamptz OR min(created_at) <= sqlc.arg(oldest_before)::timestamptz
ORDER BY min(created_at)
LIMIT sqlc.arg(row_limit);

-- name: LockNoticeGroup :many
-- まとまりの未送信の行をロックする。ほかのトランザクション（取り消しなど）がロックしている行は、待たずに飛ばす
SELECT id, record_id
FROM notification_outbox
WHERE status = 'pending' AND household_id = sqlc.arg(household_id) AND kind = sqlc.arg(kind)
  AND actor_user_id IS NOT DISTINCT FROM sqlc.narg(actor_user_id)
ORDER BY created_at, id
FOR UPDATE SKIP LOCKED;

-- name: ListNoticeCompletions :many
-- 完了通知の行の、記録と植物。記録が消えた（植物の削除など）行は含まない
SELECT o.id AS outbox_id, r.plant_id, p.name AS plant_name, r.task_kind, r.task_custom_name, r.actor_display_name
FROM notification_outbox o
JOIN care_records r ON r.id = o.record_id
JOIN plants p ON p.id = r.plant_id
WHERE o.id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY o.created_at, o.id;

-- name: GetNoticeHouseholdName :one
SELECT name FROM households WHERE id = $1;

-- name: GetNoticeUserName :one
SELECT display_name FROM users WHERE id = $1;

-- name: ListCompletionRecipients :many
-- 完了通知の宛先：完了通知を受け取るメンバーのうち、作業した本人を除いた人（要件16-1〜16-3。閲覧者を含む）
SELECT user_id FROM memberships
WHERE household_id = sqlc.arg(household_id) AND completion_notify_enabled
  AND user_id IS DISTINCT FROM sqlc.narg(actor_user_id)
ORDER BY user_id;

-- name: ListHouseholdOwners :many
-- 参加の通知の宛先：オーナー（要件3-8）
SELECT user_id FROM memberships
WHERE household_id = sqlc.arg(household_id) AND role = 'owner'
  AND user_id IS DISTINCT FROM sqlc.narg(actor_user_id)
ORDER BY user_id;

-- name: MarkNoticesSent :exec
UPDATE notification_outbox SET status = 'sent', sent_at = now()
WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: CancelNotices :exec
UPDATE notification_outbox SET status = 'canceled'
WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: FailNotices :exec
-- 送れなかった行の試行回数を増やす。上限に達したら failed にして諦める
UPDATE notification_outbox
SET attempts = attempts + 1,
    status = CASE WHEN attempts + 1 >= sqlc.arg(max_attempts)::int THEN 'failed' ELSE 'pending' END
WHERE id = ANY(sqlc.arg(ids)::uuid[]);
