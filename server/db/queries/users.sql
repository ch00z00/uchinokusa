-- name: GetUserByFirebaseUID :one
SELECT * FROM users
WHERE firebase_uid = $1 AND deleted_at IS NULL;

-- name: CreateUserIfNotExists :one
-- すでに同じ firebase_uid の行があれば何もせず、行を返さない（同時に作ろうとした場合に、片方だけが作る）
INSERT INTO users (firebase_uid, display_name, auth_provider)
VALUES ($1, $2, $3)
ON CONFLICT (firebase_uid) DO NOTHING
RETURNING *;

-- name: UpdateUserDisplayName :one
UPDATE users SET display_name = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;
