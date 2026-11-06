-- タスクを作る提案に、作るタスクの周期を加える（タスク 10.1、docs/decisions/0027）
--
-- 提案を採用したときに作るタスクは、周期がないと作れない。周期はプロバイダが決めるので、提案とともに保存する。
-- 列の形は care_tasks の周期（interval_value・interval_unit・seasonal_intervals）とそろえる。
-- create_task の提案には、作業の種類と周期を必須にする

-- +goose Up
ALTER TABLE suggestions
  ADD COLUMN task_interval_value     integer CHECK (task_interval_value >= 1),
  ADD COLUMN task_interval_unit      text CHECK (task_interval_unit IN ('day', 'week', 'month')),
  ADD COLUMN task_seasonal_intervals jsonb NOT NULL DEFAULT '{}',
  ADD CONSTRAINT suggestions_create_task_schedule_check CHECK (
    action <> 'create_task' OR (task_kind IS NOT NULL AND task_interval_value IS NOT NULL AND task_interval_unit IS NOT NULL)
  );

-- +goose Down
ALTER TABLE suggestions
  DROP CONSTRAINT suggestions_create_task_schedule_check,
  DROP COLUMN task_seasonal_intervals,
  DROP COLUMN task_interval_unit,
  DROP COLUMN task_interval_value;
