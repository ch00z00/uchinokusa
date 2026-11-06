package postgres

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/advisor"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// RecentRecordLimit は、プロバイダに渡す最近の記録の件数である。
const RecentRecordLimit = 30

// SuggestionRepository は、app.SuggestionRepository の PostgreSQL の実装である。
type SuggestionRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewSuggestionRepository(pool *pgxpool.Pool) *SuggestionRepository {
	return &SuggestionRepository{pool: pool, q: sqlcgen.New(pool)}
}

// Subject は、植物と、その世帯・植物マスタ・置き場所・タスク（削除を除く）・最近の記録を返す。植物がなければ domain.ErrNotFound を返す。
func (r *SuggestionRepository) Subject(ctx context.Context, plantID uuid.UUID) (advisor.Subject, error) {
	householdID, err := r.q.GetPlantHouseholdID(ctx, plantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return advisor.Subject{}, domain.ErrNotFound
	}
	if err != nil {
		return advisor.Subject{}, fmt.Errorf("get plant household: %w", err)
	}
	plant, err := r.q.GetPlant(ctx, sqlcgen.GetPlantParams{ID: plantID, HouseholdID: householdID})
	if err != nil {
		return advisor.Subject{}, fmt.Errorf("get plant: %w", err)
	}
	household, err := r.q.GetHousehold(ctx, householdID)
	if err != nil {
		return advisor.Subject{}, fmt.Errorf("get household: %w", err)
	}
	sub := advisor.Subject{Household: toDomainHousehold(household), Plant: toDomainPlant(plant.Plant, plant.SpeciesName, plant.NextDueOn)}

	if plant.Plant.SpeciesID.Valid {
		row, err := r.q.GetAdviceSpecies(ctx, plant.Plant.SpeciesID.UUID)
		if err != nil {
			return advisor.Subject{}, fmt.Errorf("get species: %w", err)
		}
		species, err := toDomainSpecies(row)
		if err != nil {
			return advisor.Subject{}, err
		}
		sub.Species = &species
	}
	if plant.Plant.LocationID.Valid {
		row, err := r.q.GetAdviceLocation(ctx, sqlcgen.GetAdviceLocationParams{ID: plant.Plant.LocationID.UUID, HouseholdID: householdID})
		if err != nil {
			return advisor.Subject{}, fmt.Errorf("get location: %w", err)
		}
		location := toDomainLocation(row, 0)
		sub.Location = &location
	}

	tasks, err := r.q.ListCareTasks(ctx, sqlcgen.ListCareTasksParams{HouseholdID: householdID, PlantID: plantID})
	if err != nil {
		return advisor.Subject{}, fmt.Errorf("list care tasks: %w", err)
	}
	for _, row := range tasks {
		t, err := toDomainCareTask(row)
		if err != nil {
			return advisor.Subject{}, err
		}
		sub.Tasks = append(sub.Tasks, t)
	}
	records, err := r.q.ListPlantRecords(ctx, sqlcgen.ListPlantRecordsParams{HouseholdID: householdID, PlantID: plantID, RowLimit: RecentRecordLimit})
	if err != nil {
		return advisor.Subject{}, fmt.Errorf("list plant records: %w", err)
	}
	for _, row := range records {
		sub.Records = append(sub.Records, toDomainCareRecord(row))
	}
	return sub, nil
}

// Save は、提案を保存する。世帯に同じ重複防止のキーの提案があれば、保存せず created を false にする。
// 撤回した（expired の）提案なら、新しい内容で未回答に戻し、created を true にする。
func (r *SuggestionRepository) Save(ctx context.Context, s domain.Suggestion) (domain.Suggestion, bool, error) {
	seasonal, err := marshalSeasonal(s.TaskSchedule.Seasonal)
	if err != nil {
		return domain.Suggestion{}, false, err
	}
	params := sqlcgen.CreateSuggestionParams{
		HouseholdID:           s.HouseholdID,
		PlantID:               nullUUID(s.PlantID),
		TaskID:                nullUUID(s.TaskID),
		ProviderID:            s.ProviderID,
		Kind:                  string(s.Kind),
		Action:                string(s.Action),
		TaskKind:              nullIfEmpty(string(s.TaskKind)),
		TaskSeasonalIntervals: seasonal,
		ProposedDate:          pgDate(s.ProposedDate),
		ConditionText:         nullIfEmpty(s.ConditionText),
		MessageKey:            s.MessageKey,
		MessageParams:         s.MessageParams,
		ReasonKey:             s.ReasonKey,
		ReasonParams:          s.ReasonParams,
		ValidFrom:             pgDate(s.ValidFrom),
		ValidUntil:            pgDate(s.ValidUntil),
		DedupeKey:             s.DedupeKey,
	}
	if base := s.TaskSchedule.Base; base.Value > 0 {
		params.TaskIntervalValue = pgtype.Int4{Int32: int32(base.Value), Valid: true}
		params.TaskIntervalUnit = nullIfEmpty(string(base.Unit))
	}
	row, err := r.q.CreateSuggestion(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Suggestion{}, false, nil // 同じキーの、撤回していない提案がある
	}
	if err != nil {
		return domain.Suggestion{}, false, fmt.Errorf("create suggestion: %w", err)
	}
	saved, err := toDomainSuggestion(row)
	return saved, err == nil, err
}

// Retract は、プロバイダの、タスクへの未回答の提案のうち、キーが keep にないものを撤回する（状態を expired にする）。
func (r *SuggestionRepository) Retract(ctx context.Context, householdID uuid.UUID, providerID string, taskID uuid.UUID, keep []string) error {
	if keep == nil {
		keep = []string{} // NULL の配列では <> ALL が NULL になり、何も撤回しないため
	}
	err := r.q.RetractSuggestions(ctx, sqlcgen.RetractSuggestionsParams{HouseholdID: householdID, ProviderID: providerID, TaskID: nullUUID(taskID), KeepKeys: keep})
	if err != nil {
		return fmt.Errorf("retract suggestions: %w", err)
	}
	return nil
}

// HouseholdOf は、提案が属する世帯の ID を返す。提案がなければ domain.ErrNotFound を返す。
func (r *SuggestionRepository) HouseholdOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	householdID, err := r.q.GetSuggestionHouseholdID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get suggestion household: %w", err)
	}
	return householdID, nil
}

// TimeZoneOf は、世帯のタイムゾーンを返す。
func (r *SuggestionRepository) TimeZoneOf(ctx context.Context, householdID uuid.UUID) (string, error) {
	tz, err := r.q.GetHouseholdTimeZone(ctx, householdID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get household time zone: %w", err)
	}
	return tz, nil
}

// ListActive は、on の日に有効な提案を、作った順に返す。
func (r *SuggestionRepository) ListActive(ctx context.Context, householdID uuid.UUID, on civil.Date) ([]domain.Suggestion, error) {
	rows, err := r.q.ListActiveSuggestions(ctx, sqlcgen.ListActiveSuggestionsParams{HouseholdID: householdID, OnDate: pgDate(on)})
	if err != nil {
		return nil, fmt.Errorf("list active suggestions: %w", err)
	}
	return toDomainSuggestions(rows)
}

// Accept は、提案の行をロックして採用する。タスクを作る提案なら、同じトランザクションでタスクを作る。
// 同時の採用は行ロックで順番に処理され、後のほうは採用済みの提案を見て already_responded になる。
func (r *SuggestionRepository) Accept(ctx context.Context, householdID, id, actorID uuid.UUID, in domain.AcceptInput, today civil.Date) (domain.Suggestion, domain.CareTask, error) {
	var (
		suggestion domain.Suggestion
		task       domain.CareTask
	)
	err := r.respond(ctx, householdID, id, actorID, domain.SuggestionAccepted, func(q *sqlcgen.Queries, s domain.Suggestion) error {
		plan, err := domain.PlanAcceptance(s, in, today)
		if err != nil {
			return err
		}
		if plan.Task != nil {
			if task, err = createCareTask(ctx, q, householdID, s.PlantID, *plan.Task); err != nil {
				return err
			}
		}
		return nil
	}, &suggestion)
	return suggestion, task, err
}

// Dismiss は、提案の行をロックして見送る。
func (r *SuggestionRepository) Dismiss(ctx context.Context, householdID, id, actorID uuid.UUID) (domain.Suggestion, error) {
	var suggestion domain.Suggestion
	err := r.respond(ctx, householdID, id, actorID, domain.SuggestionDismissed, func(_ *sqlcgen.Queries, s domain.Suggestion) error {
		return domain.CheckDismiss(s)
	}, &suggestion)
	return suggestion, err
}

// respond は、提案の行をロックし、apply（規則の確認と、採用で作るタスク）のあとに、対応を記録する。
func (r *SuggestionRepository) respond(ctx context.Context, householdID, id, actorID uuid.UUID, status domain.SuggestionStatus,
	apply func(*sqlcgen.Queries, domain.Suggestion) error, out *domain.Suggestion) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		row, err := q.LockSuggestion(ctx, sqlcgen.LockSuggestionParams{ID: id, HouseholdID: householdID})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock suggestion: %w", err)
		}
		current, err := toDomainSuggestion(row)
		if err != nil {
			return err
		}
		if err := apply(q, current); err != nil {
			return err
		}
		updated, err := q.RespondSuggestion(ctx, sqlcgen.RespondSuggestionParams{ID: id, Status: string(status), RespondedBy: nullUUID(actorID)})
		if err != nil {
			return fmt.Errorf("respond suggestion: %w", err)
		}
		*out, err = toDomainSuggestion(updated)
		return err
	})
}

func toDomainSuggestions(rows []sqlcgen.Suggestion) ([]domain.Suggestion, error) {
	out := make([]domain.Suggestion, 0, len(rows))
	for _, row := range rows {
		s, err := toDomainSuggestion(row)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func toDomainSuggestion(row sqlcgen.Suggestion) (domain.Suggestion, error) {
	seasonal, err := unmarshalSeasonal(row.TaskSeasonalIntervals)
	if err != nil {
		return domain.Suggestion{}, fmt.Errorf("suggestion %s: unmarshal seasonal intervals: %w", row.ID, err)
	}
	s := domain.Suggestion{
		ID:            row.ID,
		HouseholdID:   row.HouseholdID,
		PlantID:       row.PlantID.UUID,
		TaskID:        row.TaskID.UUID,
		ProviderID:    row.ProviderID,
		Kind:          domain.SuggestionKind(row.Kind),
		Action:        domain.SuggestionAction(row.Action),
		TaskKind:      domain.CareKind(emptyIfNull(row.TaskKind)),
		TaskSchedule:  domain.Schedule{Seasonal: seasonal},
		ProposedDate:  civilDate(row.ProposedDate),
		ConditionText: emptyIfNull(row.ConditionText),
		MessageKey:    row.MessageKey,
		MessageParams: row.MessageParams,
		ReasonKey:     row.ReasonKey,
		ReasonParams:  row.ReasonParams,
		ValidFrom:     civilDate(row.ValidFrom),
		ValidUntil:    civilDate(row.ValidUntil),
		DedupeKey:     row.DedupeKey,
		Status:        domain.SuggestionStatus(row.Status),
		RespondedBy:   row.RespondedBy,
		CreatedAt:     row.CreatedAt,
	}
	if row.TaskIntervalValue.Valid {
		s.TaskSchedule.Base = domain.Interval{Value: int(row.TaskIntervalValue.Int32), Unit: domain.IntervalUnit(emptyIfNull(row.TaskIntervalUnit))}
	}
	if row.RespondedAt != nil {
		s.RespondedAt = *row.RespondedAt
	}
	return s, nil
}
