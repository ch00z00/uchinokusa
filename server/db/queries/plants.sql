-- name: ListPlants :many
-- pattern は '%検索語%'（検索語の % と _ はエスケープ済み）。検索しないときは '%'
-- next_due_on は、有効なタスク（一時停止・削除したものを除く）のいちばん近い次回予定日。タスクがなければ NULL
SELECT sqlc.embed(plants), species.name_ja AS species_name,
  (SELECT min(care_tasks.next_due_on) FROM care_tasks
    WHERE care_tasks.plant_id = plants.id AND care_tasks.status = 'active' AND care_tasks.deleted_at IS NULL)::date AS next_due_on
FROM plants
LEFT JOIN species ON species.id = plants.species_id
WHERE plants.household_id = sqlc.arg(household_id)
  AND (plants.archived_at IS NOT NULL) = sqlc.arg(archived)::boolean
  AND (sqlc.narg(location_id)::uuid IS NULL OR plants.location_id = sqlc.narg(location_id)::uuid)
  AND plants.name_search LIKE sqlc.arg(pattern)::text
ORDER BY plants.created_at, plants.id;

-- name: GetPlant :one
-- 項目は ListPlants と同じ
SELECT sqlc.embed(plants), species.name_ja AS species_name,
  (SELECT min(care_tasks.next_due_on) FROM care_tasks
    WHERE care_tasks.plant_id = plants.id AND care_tasks.status = 'active' AND care_tasks.deleted_at IS NULL)::date AS next_due_on
FROM plants
LEFT JOIN species ON species.id = plants.species_id
WHERE plants.id = $1 AND plants.household_id = $2;

-- name: GetPlantHouseholdID :one
-- 植物の ID だけで呼ばれる API（/plants/{id}）で、権限を確かめる世帯を探す
SELECT household_id FROM plants WHERE id = $1;

-- name: CreatePlant :one
-- 置き場所は (location_id, household_id) の外部キーで、同じ世帯のものに限られる
INSERT INTO plants (household_id, name, name_search, species_id, species_free_text, location_id, acquired_on)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: UpdatePlant :one
-- name・name_search は NULL なら変えない。ほかの項目は set_* が true のときだけ変える（NULL を入れると未設定に戻る）
-- 種類は、マスタと自由入力をまとめて置き換える
UPDATE plants SET
  name              = COALESCE(sqlc.narg(name)::text, name),
  name_search       = COALESCE(sqlc.narg(name_search)::text, name_search),
  species_id        = CASE WHEN sqlc.arg(set_species)::boolean THEN sqlc.narg(species_id)::uuid ELSE species_id END,
  species_free_text = CASE WHEN sqlc.arg(set_species)::boolean THEN sqlc.narg(species_free_text)::text ELSE species_free_text END,
  location_id       = CASE WHEN sqlc.arg(set_location)::boolean THEN sqlc.narg(location_id)::uuid ELSE location_id END,
  acquired_on       = CASE WHEN sqlc.arg(set_acquired_on)::boolean THEN sqlc.narg(acquired_on)::date ELSE acquired_on END
WHERE id = sqlc.arg(id) AND household_id = sqlc.arg(household_id)
RETURNING id;

-- name: SetPlantArchived :one
-- アーカイブし直しても、最初にアーカイブした日時を残す
UPDATE plants SET
  archived_at = CASE WHEN sqlc.arg(archived)::boolean THEN COALESCE(archived_at, now()) END
WHERE id = sqlc.arg(id) AND household_id = sqlc.arg(household_id)
RETURNING id;

-- name: DeletePlant :execrows
-- タスク・記録・メモ・提案は、外部キーの ON DELETE CASCADE でいっしょに消える（要件5-7）
DELETE FROM plants WHERE id = $1 AND household_id = $2;
