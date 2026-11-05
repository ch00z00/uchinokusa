-- 取り消しで戻す先に、予定日を手で変えた印を加える（タスク 8.3、docs/decisions/0022）
--
-- 完了は印を外し、スキップは印を立てるので、取り消すときに記録する前の値に戻せるようにする。
-- 追加の記録はタスクを変えないので、prev_next_due_on・prev_cycle_seq と同じく NULL にする

-- +goose Up
ALTER TABLE care_records ADD COLUMN prev_due_overridden boolean;

-- +goose Down
ALTER TABLE care_records DROP COLUMN prev_due_overridden;
