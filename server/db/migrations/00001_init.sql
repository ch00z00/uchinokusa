-- MVP の初期スキーマ。設計：.kiro/specs/household-plant-care-mvp/design.md「物理データモデル」
--
-- 共通の取り決め
-- - 主キーは UUIDv7（作成順に並ぶため、インデックスが断片化しにくい）。PostgreSQL 16 には生成する関数がないので、下で定義する
-- - 列挙値は text + CHECK 制約で表す（値の追加が ALTER TABLE だけで済み、型の作り直しが要らない）
-- - 世帯の配下のテーブルは household_id を持ち、親への外部キーを (id, household_id) の組で張る。
--   これにより、別の世帯の植物やタスクを参照する行は、DB が作らせない
-- - attributes jsonb は、MVP の画面で使わない項目や将来の項目を入れる場所（要件19-8）

-- +goose Up

-- +goose StatementBegin
-- uuid_generate_v7 は、先頭48ビットにミリ秒の時刻を入れた UUID（RFC 9562 のバージョン7）を返す。
-- ランダムな UUIDv4 を作り、先頭6バイトを時刻で上書きして、バージョンのビットを 0111 にする。
CREATE FUNCTION uuid_generate_v7() RETURNS uuid
LANGUAGE sql VOLATILE
AS $$
  SELECT encode(
    set_bit(
      set_bit(
        overlay(uuid_send(gen_random_uuid())
          PLACING substring(int8send(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint) FROM 3)
          FROM 1 FOR 6),
        52, 1),
      53, 1),
    'hex')::uuid
$$;
-- +goose StatementEnd

-- +goose StatementBegin
-- set_updated_at は、行を更新したときに updated_at を現在時刻にするトリガー関数である。
CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END
$$;
-- +goose StatementEnd

-- ユーザー（Firebase Auth のユーザーと1対1）
CREATE TABLE users (
  id            uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  firebase_uid  text NOT NULL UNIQUE,
  display_name  text NOT NULL,
  auth_provider text NOT NULL,           -- Firebase の sign_in_provider（apple.com、google.com など）
  deleted_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

-- プッシュ通知の送信先となる端末
CREATE TABLE user_devices (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  fcm_token    text NOT NULL UNIQUE,      -- 別のユーザーで登録されたら付け替える（要件1-5）
  platform     text NOT NULL CHECK (platform IN ('ios', 'android')),
  time_zone    text NOT NULL,             -- IANA のタイムゾーン（Asia/Tokyo など）
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_devices_user_id_idx ON user_devices (user_id);

-- 世帯
CREATE TABLE households (
  id                uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  name              text NOT NULL,
  region_prefecture text,
  region_city       text,
  time_zone         text NOT NULL DEFAULT 'Asia/Tokyo',
  hemisphere        text NOT NULL DEFAULT 'north' CHECK (hemisphere IN ('north', 'south')),
  version           bigint NOT NULL DEFAULT 0, -- 変更のたびに増やし、リアルタイム同期の通知に含める
  attributes        jsonb NOT NULL DEFAULT '{}',
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

-- 世帯のメンバー
CREATE TABLE memberships (
  id                        uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  household_id              uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
  user_id                   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  role                      text NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
  reminder_enabled          boolean NOT NULL DEFAULT true,
  reminder_time             time NOT NULL DEFAULT '08:00',
  completion_notify_enabled boolean NOT NULL DEFAULT true,
  created_at                timestamptz NOT NULL DEFAULT now(),
  updated_at                timestamptz NOT NULL DEFAULT now(),
  UNIQUE (household_id, user_id)
);
CREATE INDEX memberships_user_id_idx ON memberships (user_id);

-- 招待（有効期限内は複数人が使える）
CREATE TABLE invitations (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
  role         text NOT NULL CHECK (role IN ('editor', 'viewer')),
  token_hash   bytea NOT NULL UNIQUE,     -- トークンの SHA-256。トークン自体は保存しない
  code         text NOT NULL UNIQUE,      -- 紛らわしい文字を除いた8文字
  expires_at   timestamptz NOT NULL DEFAULT now() + interval '7 days',
  revoked_at   timestamptz,
  created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invitations_household_id_idx ON invitations (household_id);

-- 植物マスタ（全ユーザー共通）。名前の検索インデックスは、検索のクエリとあわせてタスク 6.1 で作る
CREATE TABLE species (
  id               uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  name_ja          text NOT NULL,
  scientific_name  text,
  aliases          text[] NOT NULL DEFAULT '{}',
  light_level      text,
  min_temp_c       numeric(4, 1),
  humidity_level   text,
  airflow_need     text,
  water_tendency   text,
  outdoor_ok       boolean,
  recommended_care jsonb NOT NULL DEFAULT '{}',
  attributes       jsonb NOT NULL DEFAULT '{}', -- 出典（attributes.sources）など
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);

-- 置き場所
CREATE TABLE locations (
  id               uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  household_id     uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
  name             text NOT NULL,
  is_indoor        boolean NOT NULL DEFAULT true,
  window_direction text,
  window_distance  text,
  shading          text,
  attributes       jsonb NOT NULL DEFAULT '{}',
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, household_id) -- 子のテーブルから (id, household_id) で参照するため
);
CREATE INDEX locations_household_id_idx ON locations (household_id);

-- 植物
CREATE TABLE plants (
  id                uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  household_id      uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
  name              text NOT NULL,
  species_id        uuid REFERENCES species (id) ON DELETE SET NULL,
  species_free_text text,
  location_id       uuid,
  acquired_on       date,
  archived_at       timestamptz,
  attributes        jsonb NOT NULL DEFAULT '{}',
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, household_id),
  -- 置き場所を削除したら、植物は「置き場所未設定」にする（要件7-4）。household_id は残すため、列を指定する
  FOREIGN KEY (location_id, household_id) REFERENCES locations (id, household_id)
    ON DELETE SET NULL (location_id)
);
CREATE INDEX plants_household_id_archived_at_idx ON plants (household_id, archived_at);
CREATE INDEX plants_location_id_idx ON plants (location_id);

-- お手入れタスク。削除は deleted_at を立てる論理削除とし、記録を残す（要件8-10）
CREATE TABLE care_tasks (
  id                 uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  household_id       uuid NOT NULL,
  plant_id           uuid NOT NULL,
  kind               text NOT NULL CHECK (kind IN ('watering', 'fertilizing', 'repotting', 'pruning', 'other')),
  custom_name        text,
  interval_value     integer NOT NULL CHECK (interval_value >= 1),
  interval_unit      text NOT NULL CHECK (interval_unit IN ('day', 'week', 'month')),
  seasonal_intervals jsonb NOT NULL DEFAULT '{}', -- 例：{"winter": {"value": 2, "unit": "week"}}
  condition_text     text,                         -- 実行条件（例：土の表面が乾いていたら）
  note               text,
  next_due_on        date,
  due_overridden     boolean NOT NULL DEFAULT false,
  cycle_seq          integer NOT NULL DEFAULT 1,   -- 現在の予定の回の番号。完了するたびに1つ進める
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused')),
  deleted_at         timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, household_id),
  FOREIGN KEY (plant_id, household_id) REFERENCES plants (id, household_id) ON DELETE CASCADE,
  -- 種類が「その他」のときだけ、作業名を必須にする（要件8-3）
  CHECK (kind <> 'other' OR custom_name IS NOT NULL),
  -- 有効なタスクは、必ず次回予定を持つ
  CHECK (status <> 'active' OR next_due_on IS NOT NULL)
);
CREATE INDEX care_tasks_household_status_next_due_on_idx ON care_tasks (household_id, status, next_due_on)
  WHERE deleted_at IS NULL;
CREATE INDEX care_tasks_plant_id_idx ON care_tasks (plant_id);

-- お手入れの記録（完了・スキップ）
CREATE TABLE care_records (
  id                 uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  household_id       uuid NOT NULL,
  plant_id           uuid NOT NULL,
  task_id            uuid,
  task_kind          text NOT NULL,       -- タスクを削除しても表示できるように、記録の時点の値を持つ
  task_custom_name   text,
  outcome            text NOT NULL CHECK (outcome IN ('completed', 'skipped')),
  performed_on       date NOT NULL,
  performed_at       timestamptz NOT NULL DEFAULT now(),
  actor_user_id      uuid REFERENCES users (id) ON DELETE SET NULL,
  actor_display_name text NOT NULL,       -- メンバーが抜けても作業者名を残す（要件4-7）
  skip_reason_code   text CHECK (skip_reason_code IN ('soil_wet', 'bad_weather', 'plant_weak', 'other')),
  skip_reason_text   text CHECK (char_length(skip_reason_text) <= 200),
  recheck_on         date,
  cycle_seq          integer NOT NULL,
  is_extra           boolean NOT NULL DEFAULT false, -- 重複を承知で追加した記録
  prev_next_due_on   date,                -- 取り消したときに戻す値
  prev_cycle_seq     integer,
  edited_at          timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, household_id),
  FOREIGN KEY (plant_id, household_id) REFERENCES plants (id, household_id) ON DELETE CASCADE,
  FOREIGN KEY (task_id, household_id) REFERENCES care_tasks (id, household_id) ON DELETE SET NULL (task_id)
);
-- 同じ予定の回への完了（追加の記録を除く）は1件まで（要件11-4、設計の判断2）
CREATE UNIQUE INDEX care_records_one_completion_per_cycle_idx ON care_records (task_id, cycle_seq)
  WHERE outcome = 'completed' AND NOT is_extra;
CREATE INDEX care_records_plant_id_performed_on_idx ON care_records (plant_id, performed_on DESC);

-- 植物ごとのメモ
CREATE TABLE notes (
  id                  uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  household_id        uuid NOT NULL,
  plant_id            uuid NOT NULL,
  author_user_id      uuid REFERENCES users (id) ON DELETE SET NULL,
  author_display_name text NOT NULL,
  body                text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
  edited_at           timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (plant_id, household_id) REFERENCES plants (id, household_id) ON DELETE CASCADE
);
CREATE INDEX notes_plant_id_created_at_idx ON notes (plant_id, created_at DESC);

-- 助言プロバイダの提案
CREATE TABLE suggestions (
  id             uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  household_id   uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
  plant_id       uuid,
  task_id        uuid,
  provider_id    text NOT NULL,           -- 例：repotting.v1
  kind           text NOT NULL CHECK (kind IN ('do', 'avoid', 'conditional')),
  action         text NOT NULL CHECK (action IN ('create_task', 'skip_today', 'info')),
  task_kind      text,
  proposed_date  date,
  condition_text text,
  message_key    text NOT NULL,
  message_params jsonb NOT NULL DEFAULT '{}',
  reason_key     text NOT NULL,
  reason_params  jsonb NOT NULL DEFAULT '{}',
  valid_from     date NOT NULL,
  valid_until    date,
  status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'dismissed', 'expired')),
  responded_by   uuid REFERENCES users (id) ON DELETE SET NULL,
  responded_at   timestamptz,
  dedupe_key     text NOT NULL,           -- 例：repotting:initial:{plantID}
  created_at     timestamptz NOT NULL DEFAULT now(),
  -- 見送られた提案を、同じキーで作り直さない（要件17-6）
  UNIQUE (household_id, dedupe_key),
  FOREIGN KEY (plant_id, household_id) REFERENCES plants (id, household_id) ON DELETE CASCADE,
  FOREIGN KEY (task_id, household_id) REFERENCES care_tasks (id, household_id) ON DELETE SET NULL (task_id)
);

-- 通知のアウトボックス（完了通知・参加の通知）。記録と同じトランザクションで追加し、ジョブが送る
CREATE TABLE notification_outbox (
  id            uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  household_id  uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
  kind          text NOT NULL CHECK (kind IN ('completion', 'member_joined')),
  actor_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
  record_id     uuid,
  payload       jsonb NOT NULL DEFAULT '{}',
  status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'canceled', 'failed')),
  attempts      integer NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now(),
  sent_at       timestamptz,
  FOREIGN KEY (record_id, household_id) REFERENCES care_records (id, household_id) ON DELETE SET NULL (record_id)
);
CREATE INDEX notification_outbox_status_created_at_idx ON notification_outbox (status, created_at);

-- リマインダーの送信記録（メンバーごとに1日1回、要件15-2）
CREATE TABLE reminder_dispatches (
  id            uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
  membership_id uuid NOT NULL REFERENCES memberships (id) ON DELETE CASCADE,
  local_date    date NOT NULL,            -- 端末のタイムゾーンでの日付
  sent_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (membership_id, local_date)
);

-- updated_at を持つテーブルに、更新時のトリガーを付ける
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER user_devices_set_updated_at BEFORE UPDATE ON user_devices FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER households_set_updated_at BEFORE UPDATE ON households FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER memberships_set_updated_at BEFORE UPDATE ON memberships FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER species_set_updated_at BEFORE UPDATE ON species FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER locations_set_updated_at BEFORE UPDATE ON locations FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER plants_set_updated_at BEFORE UPDATE ON plants FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER care_tasks_set_updated_at BEFORE UPDATE ON care_tasks FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE reminder_dispatches;
DROP TABLE notification_outbox;
DROP TABLE suggestions;
DROP TABLE notes;
DROP TABLE care_records;
DROP TABLE care_tasks;
DROP TABLE plants;
DROP TABLE locations;
DROP TABLE species;
DROP TABLE invitations;
DROP TABLE memberships;
DROP TABLE households;
DROP TABLE user_devices;
DROP TABLE users;
DROP FUNCTION set_updated_at();
DROP FUNCTION uuid_generate_v7();
