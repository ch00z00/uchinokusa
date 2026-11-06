package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// CareTaskRepository は、care_tasks テーブルへのアクセスを提供する。
type CareTaskRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewCareTaskRepository(pool *pgxpool.Pool) *CareTaskRepository {
	return &CareTaskRepository{pool: pool, q: sqlcgen.New(pool)}
}

// TaskPlantOf は、植物の世帯・タイムゾーン・植物マスタの目安を返す。植物がなければ domain.ErrNotFound を返す。
func (r *CareTaskRepository) TaskPlantOf(ctx context.Context, plantID uuid.UUID) (app.TaskPlant, error) {
	row, err := r.q.GetTaskPlant(ctx, plantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.TaskPlant{}, domain.ErrNotFound
	}
	if err != nil {
		return app.TaskPlant{}, fmt.Errorf("get task plant: %w", err)
	}
	p := app.TaskPlant{HouseholdID: row.HouseholdID, TimeZone: row.TimeZone}
	if row.RecommendedCare != nil { // マスタと紐づかなければ NULL
		var care recommendedCareJSON
		if err := json.Unmarshal(row.RecommendedCare, &care); err != nil {
			return app.TaskPlant{}, fmt.Errorf("unmarshal species care: %w", err)
		}
		p.Care = ptrOf(care.toDomain())
	}
	return p, nil
}

// List は、植物の削除していないタスク（一時停止中を含む）を、作成の古い順に返す。
func (r *CareTaskRepository) List(ctx context.Context, householdID, plantID uuid.UUID) ([]domain.CareTask, error) {
	rows, err := r.q.ListCareTasks(ctx, sqlcgen.ListCareTasksParams{HouseholdID: householdID, PlantID: plantID})
	if err != nil {
		return nil, fmt.Errorf("list care tasks: %w", err)
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

// Create は、植物にタスクを作る。空文字の作業名・実行条件・メモは NULL（未設定）にする。
func (r *CareTaskRepository) Create(ctx context.Context, householdID, plantID uuid.UUID, d domain.CareTaskDraft) (domain.CareTask, error) {
	var out domain.CareTask
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		var err error
		out, err = createCareTask(ctx, q, ch, householdID, plantID, d)
		return err
	})
	return out, err
}

// createCareTask は、タスクを作り、変更イベントを足す。提案の採用（SuggestionRepository.Accept）のトランザクションの中でも使う。
func createCareTask(ctx context.Context, q *sqlcgen.Queries, ch *changes, householdID, plantID uuid.UUID, d domain.CareTaskDraft) (domain.CareTask, error) {
	seasonal, err := marshalSeasonal(d.Schedule.Seasonal)
	if err != nil {
		return domain.CareTask{}, err
	}
	row, err := q.CreateCareTask(ctx, sqlcgen.CreateCareTaskParams{
		HouseholdID:       householdID,
		PlantID:           plantID,
		Kind:              string(d.Kind),
		CustomName:        nullIfEmpty(d.CustomName),
		IntervalValue:     int32(d.Schedule.Base.Value),
		IntervalUnit:      string(d.Schedule.Base.Unit),
		SeasonalIntervals: seasonal,
		ConditionText:     nullIfEmpty(d.ConditionText),
		Note:              nullIfEmpty(d.Note),
		NextDueOn:         pgDate(d.NextDueOn),
	})
	if err != nil {
		return domain.CareTask{}, fmt.Errorf("create care task: %w", err)
	}
	ch.add(householdID, domain.ChangeTask, row.ID, domain.ChangeCreated, plantID)
	return toDomainCareTask(row)
}

// HouseholdOf は、タスクが属する世帯の ID を返す。タスクがない・削除済みなら domain.ErrNotFound を返す。
func (r *CareTaskRepository) HouseholdOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	householdID, err := r.q.GetCareTaskHouseholdID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get care task household: %w", err)
	}
	return householdID, nil
}

// Get は、世帯の削除していないタスクを返す。なければ domain.ErrNotFound を返す。
func (r *CareTaskRepository) Get(ctx context.Context, householdID, id uuid.UUID) (domain.CareTask, error) {
	row, err := r.q.GetCareTask(ctx, sqlcgen.GetCareTaskParams{ID: id, HouseholdID: householdID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CareTask{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CareTask{}, fmt.Errorf("get care task: %w", err)
	}
	return toDomainCareTask(row)
}

// Update は、世帯の削除していないタスクを変更する。予定日を変えた場合は、手で変えた印を立てる。
func (r *CareTaskRepository) Update(ctx context.Context, householdID, id uuid.UUID, p domain.CareTaskPatch) (domain.CareTask, error) {
	params := sqlcgen.UpdateCareTaskParams{ID: id, HouseholdID: householdID, CustomName: p.CustomName}
	if i := p.Interval; i != nil {
		params.IntervalValue = pgtype.Int4{Int32: int32(i.Value), Valid: true}
		params.IntervalUnit = ptrOf(string(i.Unit))
	}
	if p.Seasonal != nil {
		seasonal, err := marshalSeasonal(*p.Seasonal)
		if err != nil {
			return domain.CareTask{}, err
		}
		params.SeasonalIntervals = seasonal
	}
	if p.ConditionText != nil {
		params.SetConditionText, params.ConditionText = true, nullIfEmpty(*p.ConditionText)
	}
	if p.Note != nil {
		params.SetNote, params.Note = true, nullIfEmpty(*p.Note)
	}
	if p.NextDueOn != nil {
		params.NextDueOn = pgDate(*p.NextDueOn)
	}
	var out domain.CareTask
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		row, err := q.UpdateCareTask(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("update care task: %w", err)
		}
		ch.add(householdID, domain.ChangeTask, id, domain.ChangeUpdated, row.PlantID)
		out, err = toDomainCareTask(row)
		return err
	})
	return out, err
}

// Delete は、世帯のタスクを論理削除する。記録の task_id は残り、履歴として表示できる。
func (r *CareTaskRepository) Delete(ctx context.Context, householdID, id uuid.UUID) error {
	return withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		plantID, err := q.DeleteCareTask(ctx, sqlcgen.DeleteCareTaskParams{ID: id, HouseholdID: householdID})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("delete care task: %w", err)
		}
		ch.add(householdID, domain.ChangeTask, id, domain.ChangeDeleted, plantID)
		return nil
	})
}

// SetStatus は、世帯の削除していないタスクの状態を変える。nextDueOn が nil なら、次回予定は変えない。
func (r *CareTaskRepository) SetStatus(ctx context.Context, householdID, id uuid.UUID, status domain.TaskStatus, nextDueOn *civil.Date) (domain.CareTask, error) {
	params := sqlcgen.SetCareTaskStatusParams{ID: id, HouseholdID: householdID, Status: string(status)}
	if nextDueOn != nil {
		params.NextDueOn = pgDate(*nextDueOn)
	}
	var out domain.CareTask
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		row, err := q.SetCareTaskStatus(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("set care task status: %w", err)
		}
		ch.add(householdID, domain.ChangeTask, id, domain.ChangeUpdated, row.PlantID)
		out, err = toDomainCareTask(row)
		return err
	})
	return out, err
}

// TimeZoneOf は、世帯のタイムゾーンを返す。世帯がなければ domain.ErrNotFound を返す。
func (r *CareTaskRepository) TimeZoneOf(ctx context.Context, householdID uuid.UUID) (string, error) {
	tz, err := r.q.GetHouseholdTimeZone(ctx, householdID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get household time zone: %w", err)
	}
	return tz, nil
}

// intervalJSON は、care_tasks.seasonal_intervals（jsonb）の各季節の値の形である（例：{"winter": {"value": 2, "unit": "week"}}）。
type intervalJSON struct {
	Value int    `json:"value"`
	Unit  string `json:"unit"`
}

func marshalSeasonal(s domain.SeasonalIntervals) ([]byte, error) {
	m := make(map[string]intervalJSON, len(s))
	for season, i := range s {
		m[string(season)] = intervalJSON{Value: i.Value, Unit: string(i.Unit)}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal seasonal intervals: %w", err)
	}
	return b, nil
}

func unmarshalSeasonal(b []byte) (domain.SeasonalIntervals, error) {
	var seasonalJSON map[string]intervalJSON
	if err := json.Unmarshal(b, &seasonalJSON); err != nil {
		return nil, err
	}
	seasonal := make(domain.SeasonalIntervals, len(seasonalJSON))
	for season, i := range seasonalJSON {
		seasonal[domain.Season(season)] = domain.Interval{Value: i.Value, Unit: domain.IntervalUnit(i.Unit)}
	}
	return seasonal, nil
}

func toDomainCareTask(row sqlcgen.CareTask) (domain.CareTask, error) {
	seasonal, err := unmarshalSeasonal(row.SeasonalIntervals)
	if err != nil {
		return domain.CareTask{}, fmt.Errorf("care task %s: unmarshal seasonal intervals: %w", row.ID, err)
	}
	return domain.CareTask{
		ID:          row.ID,
		HouseholdID: row.HouseholdID,
		PlantID:     row.PlantID,
		CareTaskProfile: domain.CareTaskProfile{
			Kind:       domain.CareKind(row.Kind),
			CustomName: emptyIfNull(row.CustomName),
			Schedule: domain.Schedule{
				Base:     domain.Interval{Value: int(row.IntervalValue), Unit: domain.IntervalUnit(row.IntervalUnit)},
				Seasonal: seasonal,
			},
			ConditionText: emptyIfNull(row.ConditionText),
			Note:          emptyIfNull(row.Note),
		},
		NextDueOn:     civilDate(row.NextDueOn),
		DueOverridden: row.DueOverridden,
		CycleSeq:      int(row.CycleSeq),
		Status:        domain.TaskStatus(row.Status),
		CreatedAt:     row.CreatedAt,
	}, nil
}

func ptrOf[T any](v T) *T { return &v }
