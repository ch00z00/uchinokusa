-- name: CreateHouseholdWithOwner :one
-- 世帯と、作成したユーザーのオーナーのメンバーシップを、1つの文で作る（片方だけができることがない）
WITH h AS (
  INSERT INTO households (name, region_prefecture, region_city)
  VALUES (sqlc.arg(name), sqlc.narg(region_prefecture), sqlc.narg(region_city))
  RETURNING *
), m AS (
  INSERT INTO memberships (household_id, user_id, role)
  SELECT h.id, sqlc.arg(owner_id), 'owner' FROM h
)
SELECT * FROM h;

-- name: ListHouseholdsByUser :many
SELECT sqlc.embed(households), memberships.role
FROM households
JOIN memberships ON memberships.household_id = households.id
WHERE memberships.user_id = $1
ORDER BY households.created_at, households.id;

-- name: UpdateHousehold :one
-- name は NULL なら変えない。地域は set_* が true のときだけ変える（NULL を入れると未設定に戻る）
UPDATE households SET
  name              = COALESCE(sqlc.narg(name), name),
  region_prefecture = CASE WHEN sqlc.arg(set_prefecture)::boolean THEN sqlc.narg(region_prefecture) ELSE region_prefecture END,
  region_city       = CASE WHEN sqlc.arg(set_city)::boolean THEN sqlc.narg(region_city) ELSE region_city END
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteHousehold :exec
-- 配下のデータ（メンバー・招待・置き場所・植物・タスク・記録・メモ・提案など）は ON DELETE CASCADE で消える
DELETE FROM households WHERE id = $1;
