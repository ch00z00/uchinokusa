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
