-- name: GetMembership :one
SELECT * FROM memberships
WHERE household_id = $1 AND user_id = $2;

-- name: ListMembers :many
SELECT memberships.user_id, users.display_name, memberships.role, memberships.created_at
FROM memberships
JOIN users ON users.id = memberships.user_id
WHERE memberships.household_id = $1
ORDER BY memberships.created_at, memberships.id;

-- name: UpdateMemberRole :one
-- オーナーの行は変えない（オーナーの交代は移譲で行う）。行が返らなければ、メンバーでないかオーナーである
WITH updated AS (
  UPDATE memberships SET role = sqlc.arg(role)
  WHERE household_id = sqlc.arg(household_id) AND user_id = sqlc.arg(user_id) AND role <> 'owner'
  RETURNING user_id, role, created_at
)
SELECT updated.user_id, users.display_name, updated.role, updated.created_at
FROM updated JOIN users ON users.id = updated.user_id;

-- name: RemoveMember :execrows
-- オーナーは外さない。0行なら、メンバーでないかオーナーである
DELETE FROM memberships
WHERE household_id = $1 AND user_id = $2 AND role <> 'owner';

-- name: LockMemberships :many
-- 移譲の対象の行をロックする。ID の順にロックして、同時の移譲でのデッドロックを避ける
SELECT * FROM memberships
WHERE household_id = sqlc.arg(household_id) AND user_id = ANY(sqlc.arg(user_ids)::uuid[])
ORDER BY id
FOR UPDATE;

-- name: SetMemberRole :exec
UPDATE memberships SET role = $3
WHERE household_id = $1 AND user_id = $2;

-- name: GetNotificationSettings :one
SELECT reminder_enabled, reminder_time, completion_notify_enabled FROM memberships
WHERE household_id = $1 AND user_id = $2;

-- name: UpdateNotificationSettings :one
UPDATE memberships
SET reminder_enabled = $3, reminder_time = $4, completion_notify_enabled = $5
WHERE household_id = $1 AND user_id = $2
RETURNING reminder_enabled, reminder_time, completion_notify_enabled;
