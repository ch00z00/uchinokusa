-- name: CreateUser :one
INSERT INTO users (firebase_uid, display_name, auth_provider)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByFirebaseUID :one
SELECT * FROM users
WHERE firebase_uid = $1 AND deleted_at IS NULL;
