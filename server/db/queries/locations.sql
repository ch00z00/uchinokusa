-- name: ListLocations :many
-- 植物の数は、アーカイブしたものを除いて数える（一覧に表示される植物の数とそろえる）
SELECT sqlc.embed(locations),
  (SELECT count(*) FROM plants WHERE plants.location_id = locations.id AND plants.archived_at IS NULL) AS plant_count
FROM locations
WHERE locations.household_id = $1
ORDER BY locations.created_at, locations.id;

-- name: CreateLocation :one
INSERT INTO locations (household_id, name, is_indoor, window_direction, window_distance, shading)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetLocationHouseholdID :one
-- 置き場所の ID だけで呼ばれる API（/locations/{id}）で、権限を確かめる世帯を探す
SELECT household_id FROM locations WHERE id = $1;

-- name: UpdateLocation :one
-- name・is_indoor は NULL なら変えない。環境の項目は set_* が true のときだけ変える（NULL を入れると未設定に戻る）
UPDATE locations SET
  name             = COALESCE(sqlc.narg(name)::text, name),
  is_indoor        = COALESCE(sqlc.narg(is_indoor)::boolean, is_indoor),
  window_direction = CASE WHEN sqlc.arg(set_window_direction)::boolean THEN sqlc.narg(window_direction)::text ELSE window_direction END,
  window_distance  = CASE WHEN sqlc.arg(set_window_distance)::boolean THEN sqlc.narg(window_distance)::text ELSE window_distance END,
  shading          = CASE WHEN sqlc.arg(set_shading)::boolean THEN sqlc.narg(shading)::text ELSE shading END
WHERE locations.id = sqlc.arg(id) AND locations.household_id = sqlc.arg(household_id)
RETURNING *,
  (SELECT count(*) FROM plants WHERE plants.location_id = locations.id AND plants.archived_at IS NULL) AS plant_count;

-- name: DeleteLocation :execrows
-- 紐づく植物の location_id は、外部キーの ON DELETE SET NULL で未設定になる（要件7-4）
DELETE FROM locations WHERE id = $1 AND household_id = $2;
