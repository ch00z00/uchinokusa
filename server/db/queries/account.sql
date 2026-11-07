-- name: LockUserForDeletion :one
-- 削除済みのユーザーは対象にしない（もう一度は消せない）
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: LockUserHouseholds :many
-- 所属する世帯の行をロックする。参加（メンバーシップの追加）は世帯の行を参照するロックを取るので、
-- 参加の途中なら、それが確定するまで待つ（確かめている途中に参加したメンバーも数に入れる）
SELECT households.id, households.name
FROM households
JOIN memberships ON memberships.household_id = households.id
WHERE memberships.user_id = $1
ORDER BY households.id
FOR UPDATE OF households;

-- name: ListHouseholdMembership :many
-- 世帯ごとの、メンバーの数と、ユーザーのロール
SELECT household_id, count(*)::int AS members,
  (array_agg(role) FILTER (WHERE user_id = sqlc.arg(user_id)))[1]::text AS role
FROM memberships
WHERE household_id = ANY(sqlc.arg(household_ids)::uuid[])
GROUP BY household_id;

-- name: DeleteAccountMembership :exec
-- アカウントの削除で、世帯から外す（オーナーでも外す。ほかにメンバーのいる世帯の唯一のオーナーは、先に拒否している）
DELETE FROM memberships WHERE household_id = $1 AND user_id = $2;

-- name: AnonymizeCareRecords :exec
-- 記録の作業者名を「退会したユーザー」にし、ユーザーとのつながりも切る
UPDATE care_records SET actor_display_name = sqlc.arg(name), actor_user_id = NULL WHERE actor_user_id = sqlc.arg(user_id);

-- name: AnonymizeNotes :exec
UPDATE notes SET author_display_name = sqlc.arg(name), author_user_id = NULL WHERE author_user_id = sqlc.arg(user_id);

-- name: CancelActorNotices :exec
-- 退会したユーザーの、まだ送っていない通知は送らない
UPDATE notification_outbox SET status = 'canceled' WHERE actor_user_id = $1 AND status = 'pending';

-- name: DeleteAllUserDevices :exec
DELETE FROM user_devices WHERE user_id = $1;

-- name: MarkUserDeleted :exec
-- ユーザーの行は、削除済みの印として残し、表示名（個人情報）を消す。
-- 行ごと消すと、ほかの端末に残っている ID トークン（最大1時間有効）で呼ばれたときに、ユーザーが作り直されてしまう
UPDATE users SET display_name = '', deleted_at = now() WHERE id = $1;
