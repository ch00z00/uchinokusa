-- name: CreateInvitation :one
-- 有効期限は、列の初期値（作成から7日）にする
INSERT INTO invitations (household_id, role, token_hash, code, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListActiveInvitations :many
SELECT * FROM invitations
WHERE household_id = $1 AND revoked_at IS NULL AND expires_at > now()
ORDER BY created_at DESC, id DESC;

-- name: RevokeInvitation :execrows
-- 取り消し済みなら、最初に取り消した日時を保つ。0行なら、世帯の招待ではない
UPDATE invitations SET revoked_at = COALESCE(revoked_at, now())
WHERE household_id = $1 AND id = $2;

-- 招待は、コードかトークンのハッシュのどちらか一方で探す（もう一方は NULL にする。NULL との = は一致しない）

-- name: PreviewInvitation :one
SELECT households.name AS household_name, invitations.role, invitations.expires_at,
  EXISTS (
    SELECT 1 FROM memberships
    WHERE memberships.household_id = invitations.household_id AND memberships.user_id = sqlc.arg(user_id)
  ) AS already_member
FROM invitations
JOIN households ON households.id = invitations.household_id
WHERE (invitations.code = sqlc.narg(code) OR invitations.token_hash = sqlc.narg(token_hash))
  AND invitations.revoked_at IS NULL AND invitations.expires_at > now();

-- name: LockValidInvitation :one
-- 参加のあいだ、取り消し（UPDATE）を待たせる。取り消しが先に終わっていれば、待ったあとに条件で外れる
SELECT * FROM invitations
WHERE (code = sqlc.narg(code) OR token_hash = sqlc.narg(token_hash))
  AND revoked_at IS NULL AND expires_at > now()
FOR SHARE;

-- name: AddMemberIfAbsent :one
-- すでにメンバーなら何もせず、行を返さない
INSERT INTO memberships (household_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (household_id, user_id) DO NOTHING
RETURNING id;

-- name: EnqueueMemberJoined :exec
-- 参加の通知を予約する。送信はジョブ（タスク 12.x）が行う
INSERT INTO notification_outbox (household_id, kind, actor_user_id, payload)
VALUES ($1, 'member_joined', $2, $3);
