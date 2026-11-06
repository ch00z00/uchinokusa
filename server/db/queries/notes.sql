-- name: CreateNote :one
-- 作成者の名前は、作成の時点の値を写して持つ（記録と同じく、メンバーの退出のあとも表示するため）
INSERT INTO notes (household_id, plant_id, author_user_id, author_display_name, body)
SELECT sqlc.arg(household_id), sqlc.arg(plant_id), users.id, users.display_name, sqlc.arg(body)
FROM users
WHERE users.id = sqlc.arg(author_user_id)
RETURNING *;

-- name: GetNoteHouseholdID :one
-- 権限を確かめるため、メモが属する世帯を探す
SELECT household_id FROM notes WHERE id = $1;

-- name: GetNote :one
SELECT * FROM notes WHERE id = $1 AND household_id = $2;

-- name: UpdateNoteBody :one
-- 本文が変わったときだけ、編集した日時を付ける（要件13-3）。右辺の body は、変える前の値
UPDATE notes
SET body = sqlc.arg(body), edited_at = CASE WHEN body = sqlc.arg(body) THEN edited_at ELSE now() END
WHERE id = sqlc.arg(id) AND household_id = sqlc.arg(household_id)
RETURNING *;

-- name: DeleteNote :one
-- 変更イベントに入れるため、消したメモの植物を返す
DELETE FROM notes WHERE id = $1 AND household_id = $2 RETURNING plant_id;

-- name: ListPlantNotes :many
-- 植物のメモを、作成日時・ID の新しい順に返す（要件13-2）。編集しても並びは変えない
-- 次のページは、前のページの最後のメモの位置（cursor）より後から始める
SELECT * FROM notes
WHERE household_id = sqlc.arg(household_id) AND plant_id = sqlc.arg(plant_id)
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL
    OR (created_at, id) < (sqlc.narg(after_created_at), sqlc.narg(after_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);
