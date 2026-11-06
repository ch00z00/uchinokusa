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

// TodayRepository は、app.TodayRepository の PostgreSQL の実装である。
type TodayRepository struct {
	q *sqlcgen.Queries
}

// NewTodayRepository は、TodayRepository を作る。
func NewTodayRepository(pool *pgxpool.Pool) *TodayRepository {
	return &TodayRepository{q: sqlcgen.New(pool)}
}

// TimeZoneOf は、世帯のタイムゾーンを返す。
func (r *TodayRepository) TimeZoneOf(ctx context.Context, householdID uuid.UUID) (string, error) {
	tz, err := r.q.GetHouseholdTimeZone(ctx, householdID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get household time zone: %w", err)
	}
	return tz, nil
}

// Due は、予定日が today 以前の、有効なタスク（アーカイブしていない植物のもの）を、予定日の古い順に返す（要件9-1）。
func (r *TodayRepository) Due(ctx context.Context, householdID uuid.UUID, today civil.Date) ([]domain.TodayTask, error) {
	rows, err := r.q.ListDueTasks(ctx, sqlcgen.ListDueTasksParams{HouseholdID: householdID, Today: pgDate(today)})
	if err != nil {
		return nil, fmt.Errorf("list due tasks: %w", err)
	}
	out := make([]domain.TodayTask, len(rows))
	for i, row := range rows {
		if out[i], err = toTodayTask(row.CareTask, row.PlantName, row.LocationID, row.LocationName); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Next は、予定日が today より後の、直近の有効なタスクを返す（要件9-6）。なければ ok が false になる。
func (r *TodayRepository) Next(ctx context.Context, householdID uuid.UUID, today civil.Date) (task domain.TodayTask, ok bool, err error) {
	row, err := r.q.GetNextTask(ctx, sqlcgen.GetNextTaskParams{HouseholdID: householdID, Today: pgDate(today)})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TodayTask{}, false, nil
	}
	if err != nil {
		return domain.TodayTask{}, false, fmt.Errorf("get next task: %w", err)
	}
	task, err = toTodayTask(row.CareTask, row.PlantName, row.LocationID, row.LocationName)
	return task, err == nil, err
}

// TaskSuggestions は、タスクへの、today の時点で有効な「やらない方がよい」「条件つき」の提案を返す（要件9-4）。
func (r *TodayRepository) TaskSuggestions(ctx context.Context, householdID uuid.UUID, taskIDs []uuid.UUID, today civil.Date) ([]domain.Suggestion, error) {
	rows, err := r.q.ListActiveTaskSuggestions(ctx, sqlcgen.ListActiveTaskSuggestionsParams{HouseholdID: householdID, TaskIds: taskIDs, Today: pgDate(today)})
	if err != nil {
		return nil, fmt.Errorf("list active task suggestions: %w", err)
	}
	return toDomainSuggestions(rows)
}

func toTodayTask(t sqlcgen.CareTask, plantName string, locationID uuid.NullUUID, locationName *string) (domain.TodayTask, error) {
	task, err := toDomainCareTask(t)
	if err != nil {
		return domain.TodayTask{}, err
	}
	return domain.TodayTask{Task: task, PlantName: plantName, LocationID: locationID.UUID, LocationName: emptyIfNull(locationName)}, nil
}
