-- 置き場所の環境の項目の取りうる値（タスク 6.2、docs/decisions/0016）
--
-- 列は 00001 で text として作っておいた。値はアプリの検証（domain/location.go）と同じものを CHECK 制約でも決める。
-- NULL は「未設定（わからない）」を表す

-- +goose Up
ALTER TABLE locations
  ADD CONSTRAINT locations_window_direction_check CHECK (window_direction IN ('n', 'ne', 'e', 'se', 's', 'sw', 'w', 'nw')),
  ADD CONSTRAINT locations_window_distance_check  CHECK (window_distance IN ('near', 'middle', 'far')),
  ADD CONSTRAINT locations_shading_check          CHECK (shading IN ('none', 'sheer'));

-- +goose Down
ALTER TABLE locations
  DROP CONSTRAINT locations_shading_check,
  DROP CONSTRAINT locations_window_distance_check,
  DROP CONSTRAINT locations_window_direction_check;
