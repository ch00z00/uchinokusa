package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// CalendarRepository は、app.CalendarRepository の PostgreSQL の実装である。
type CalendarRepository struct {
	q *sqlcgen.Queries
}

func NewCalendarRepository(pool *pgxpool.Pool) *CalendarRepository {
	return &CalendarRepository{q: sqlcgen.New(pool)}
}

// CalendarPlant は、植物の世帯・タイムゾーン・半球と、アーカイブしたかを返す。植物がなければ domain.ErrNotFound を返す。
func (r *CalendarRepository) CalendarPlant(ctx context.Context, plantID uuid.UUID) (app.CalendarPlant, error) {
	row, err := r.q.GetCalendarPlant(ctx, plantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.CalendarPlant{}, domain.ErrNotFound
	}
	if err != nil {
		return app.CalendarPlant{}, fmt.Errorf("get calendar plant: %w", err)
	}
	return app.CalendarPlant{HouseholdID: row.HouseholdID, TimeZone: row.TimeZone, Hemisphere: domain.Hemisphere(row.Hemisphere), Archived: row.Archived}, nil
}

// Records は、実施日が first〜last の記録を、実施日・記録した順に返す。
func (r *CalendarRepository) Records(ctx context.Context, householdID, plantID uuid.UUID, first, last civil.Date) ([]domain.CareRecord, error) {
	rows, err := r.q.ListMonthRecords(ctx, sqlcgen.ListMonthRecordsParams{HouseholdID: householdID, PlantID: plantID, FirstDay: pgDate(first), LastDay: pgDate(last)})
	if err != nil {
		return nil, fmt.Errorf("list month records: %w", err)
	}
	out := make([]domain.CareRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainCareRecord(row))
	}
	return out, nil
}

// ActiveTasks は、植物の有効なタスク（一時停止・削除を除く）を、作業の種類の順に返す。
func (r *CalendarRepository) ActiveTasks(ctx context.Context, householdID, plantID uuid.UUID) ([]domain.CareTask, error) {
	rows, err := r.q.ListPlantActiveTasks(ctx, sqlcgen.ListPlantActiveTasksParams{HouseholdID: householdID, PlantID: plantID})
	if err != nil {
		return nil, fmt.Errorf("list plant active tasks: %w", err)
	}
	out := make([]domain.CareTask, 0, len(rows))
	for _, row := range rows {
		t, err := toDomainCareTask(row)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// Notes は、作成日時が [startsAt, endsAt) のメモを、古い順に返す。
func (r *CalendarRepository) Notes(ctx context.Context, householdID, plantID uuid.UUID, startsAt, endsAt time.Time) ([]domain.Note, error) {
	rows, err := r.q.ListMonthNotes(ctx, sqlcgen.ListMonthNotesParams{HouseholdID: householdID, PlantID: plantID, StartsAt: startsAt, EndsAt: endsAt})
	if err != nil {
		return nil, fmt.Errorf("list month notes: %w", err)
	}
	out := make([]domain.Note, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainNote(row))
	}
	return out, nil
}
