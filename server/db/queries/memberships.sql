-- name: GetMembership :one
SELECT * FROM memberships
WHERE household_id = $1 AND user_id = $2;
