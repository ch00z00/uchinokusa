-- 植物の名前の検索と、種類の入れ方の制約（タスク 6.3、docs/decisions/0017）
--
-- - name_search：植物の名前を検索用に正規化したもの。登録・変更のときにアプリが作る（植物マスタの search_text と同じ理由。00002）
--   世帯の植物は多くても数百なので、索引は世帯の条件（plants_household_id_archived_at_idx）に任せ、部分一致の索引は作らない
-- - 種類は、植物マスタ（species_id）か自由入力（species_free_text）のどちらか一方だけを持つ（要件6-3）

-- +goose Up
ALTER TABLE plants
  ADD COLUMN name_search text NOT NULL DEFAULT '',
  ADD CONSTRAINT plants_species_one_of_check CHECK (species_id IS NULL OR species_free_text IS NULL);

-- +goose Down
ALTER TABLE plants
  DROP CONSTRAINT plants_species_one_of_check,
  DROP COLUMN name_search;
