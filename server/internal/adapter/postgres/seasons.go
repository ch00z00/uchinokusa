package postgres

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// SeasonRepository は、季節の切り替えで、タスクの次回予定を計算し直す（job.SeasonStore）。
type SeasonRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewSeasonRepository(pool *pgxpool.Pool) *SeasonRepository {
	return &SeasonRepository{pool: pool, q: sqlcgen.New(pool)}
}

// Households は、すべての世帯の ID・タイムゾーン・半球を返す。
func (r *SeasonRepository) Households(ctx context.Context) ([]domain.Household, error) {
	rows, err := r.q.ListHouseholdCalendars(ctx)
	if err != nil {
		return nil, fmt.Errorf("list household calendars: %w", err)
	}
	out := make([]domain.Household, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Household{ID: row.ID, TimeZone: row.TimeZone, Hemisphere: domain.Hemisphere(row.Hemisphere)})
	}
	return out, nil
}

// Recalculate は、1つのトランザクションで、世帯の季節ごとの周期を持つ有効なタスクをロックし、
// 最後の完了に seasonStart の季節の周期を足した日（domain.Schedule.NextDueAtSeasonStart）に次回予定を直す。
//   - 起点は、最後の完了（追加の記録を除く。docs/decisions/0023）。一度も完了していなければ変えない
//     （最後の記録がスキップなら、確認日の印 due_overridden が立っているので、ロックの時点で対象から外れる）
//   - 値が変わるタスクだけを更新し、変更イベントを出す。初日のうちに何度実行しても同じ結果になる
//   - 完了と同じくタスクの行をロックするので、完了と重なっても、確定した完了を起点に計算する
func (r *SeasonRepository) Recalculate(ctx context.Context, householdID uuid.UUID, seasonStart civil.Date, h domain.Hemisphere) (int, error) {
	changed := 0
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		rows, err := q.LockSeasonalTasks(ctx, householdID)
		if err != nil {
			return fmt.Errorf("lock seasonal tasks: %w", err)
		}
		for _, row := range rows {
			last, err := q.GetLastCompletionOn(ctx, nullUUID(row.ID))
			if errors.Is(err, pgx.ErrNoRows) {
				continue // 一度も完了していない
			}
			if err != nil {
				return fmt.Errorf("get last completion: %w", err)
			}
			task, err := toDomainCareTask(row)
			if err != nil {
				return err
			}
			next := task.Schedule.NextDueAtSeasonStart(civilDate(last), seasonStart, h)
			if next == task.NextDueOn {
				continue
			}
			if err := q.SetSeasonalNextDue(ctx, sqlcgen.SetSeasonalNextDueParams{ID: row.ID, NextDueOn: pgDate(next)}); err != nil {
				return fmt.Errorf("set seasonal next due: %w", err)
			}
			ch.add(householdID, domain.ChangeTask, row.ID, domain.ChangeUpdated, row.PlantID)
			changed++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return changed, nil
}
