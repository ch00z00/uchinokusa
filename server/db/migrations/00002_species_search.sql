-- 植物マスタの投入と検索のための列・索引（タスク 6.1、docs/decisions/0015）
--
-- - slug：初期データ（db/seeds/species.yaml）の各行を指す固定の名前。投入のときに、これで追加・更新を決める
-- - search_text：和名・学名・別名を検索用に正規化してつないだもの。投入のときにアプリが作る
--   （ひらがなをカタカナに、全角・半角や大文字・小文字をそろえる処理は SQL では書きにくいため）
-- - 性質の列は、取りうる値を CHECK 制約で決める。NULL は「出典が見つからず、わからない」を表す

-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE species
  ADD COLUMN slug        text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  ADD COLUMN search_text text NOT NULL DEFAULT '',
  ADD CONSTRAINT species_light_level_check    CHECK (light_level IN ('low', 'medium', 'high')),
  ADD CONSTRAINT species_humidity_level_check CHECK (humidity_level IN ('low', 'medium', 'high')),
  ADD CONSTRAINT species_airflow_need_check   CHECK (airflow_need IN ('low', 'medium', 'high')),
  ADD CONSTRAINT species_water_tendency_check CHECK (water_tendency IN ('less', 'moderate', 'more'));

-- 部分一致（LIKE '%…%'）を速くする索引
CREATE INDEX species_search_text_trgm_idx ON species USING gin (search_text gin_trgm_ops);

-- +goose Down
DROP INDEX species_search_text_trgm_idx;
ALTER TABLE species
  DROP CONSTRAINT species_water_tendency_check,
  DROP CONSTRAINT species_airflow_need_check,
  DROP CONSTRAINT species_humidity_level_check,
  DROP CONSTRAINT species_light_level_check,
  DROP COLUMN search_text,
  DROP COLUMN slug;
DROP EXTENSION pg_trgm;
