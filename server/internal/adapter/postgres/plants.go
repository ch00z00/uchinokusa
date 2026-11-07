package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

const (
	// pgForeignKeyViolation は、外部キーの制約の違反を表す SQLSTATE である。
	pgForeignKeyViolation = "23503"
	// 植物の種類・置き場所の外部キーの制約の名前（PostgreSQL が既定で付ける名前）
	plantsSpeciesFKey  = "plants_species_id_fkey"
	plantsLocationFKey = "plants_location_id_household_id_fkey"
)

// PlantRepository は、plants テーブルへのアクセスを提供する。
type PlantRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewPlantRepository(pool *pgxpool.Pool) *PlantRepository {
	return &PlantRepository{pool: pool, q: sqlcgen.New(pool)}
}

// List は、世帯の植物のうち、条件に合うものを登録の古い順に返す。
func (r *PlantRepository) List(ctx context.Context, householdID uuid.UUID, filter domain.PlantFilter) ([]domain.Plant, error) {
	rows, err := r.q.ListPlants(ctx, sqlcgen.ListPlantsParams{
		HouseholdID: householdID,
		Archived:    filter.Archived,
		LocationID:  nullUUID(filter.LocationID),
		Pattern:     "%" + escapeLike(filter.Query) + "%",
	})
	if err != nil {
		return nil, fmt.Errorf("list plants: %w", err)
	}
	out := make([]domain.Plant, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainPlant(row.Plant, row.SpeciesName, row.NextDueOn))
	}
	return out, nil
}

// Create は、世帯に植物を登録し、マスタの和名と次回予定日を含めて返す。
// 種類・置き場所が存在しない（置き場所はほかの世帯のものも含む）場合は、domain.ValidationError を返す。
func (r *PlantRepository) Create(ctx context.Context, householdID uuid.UUID, p domain.PlantProfile) (domain.Plant, error) {
	var out domain.Plant
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		id, err := q.CreatePlant(ctx, sqlcgen.CreatePlantParams{
			HouseholdID:     householdID,
			Name:            p.Name,
			NameSearch:      domain.NormalizeSearchText(p.Name),
			SpeciesID:       nullUUID(p.Species.ID),
			SpeciesFreeText: nullIfEmpty(p.Species.FreeText),
			LocationID:      nullUUID(p.LocationID),
			AcquiredOn:      pgDate(p.AcquiredOn),
		})
		if err != nil {
			return plantWriteError("create plant", err)
		}
		ch.add(householdID, domain.ChangePlant, id, domain.ChangeCreated, id)
		out, err = getPlant(ctx, q, householdID, id)
		return err
	})
	return out, err
}

// HouseholdOf は、植物が属する世帯の ID を返す。植物がなければ domain.ErrNotFound を返す。
func (r *PlantRepository) HouseholdOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	householdID, err := r.q.GetPlantHouseholdID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get plant household: %w", err)
	}
	return householdID, nil
}

// Get は、世帯の植物を返す。世帯の植物でなければ domain.ErrNotFound を返す。
func (r *PlantRepository) Get(ctx context.Context, householdID, id uuid.UUID) (domain.Plant, error) {
	return getPlant(ctx, r.q, householdID, id)
}

func getPlant(ctx context.Context, q *sqlcgen.Queries, householdID, id uuid.UUID) (domain.Plant, error) {
	row, err := q.GetPlant(ctx, sqlcgen.GetPlantParams{ID: id, HouseholdID: householdID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Plant{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Plant{}, fmt.Errorf("get plant: %w", err)
	}
	return toDomainPlant(row.Plant, row.SpeciesName, row.NextDueOn), nil
}

// Update は、世帯の植物を変更する。世帯の植物でなければ domain.ErrNotFound を返す。
func (r *PlantRepository) Update(ctx context.Context, householdID, id uuid.UUID, p domain.PlantPatch) (domain.Plant, error) {
	params := sqlcgen.UpdatePlantParams{ID: id, HouseholdID: householdID, Name: p.Name}
	if p.Name != nil {
		nameSearch := domain.NormalizeSearchText(*p.Name)
		params.NameSearch = &nameSearch
	}
	if p.Species != nil {
		params.SetSpecies, params.SpeciesID, params.SpeciesFreeText = true, nullUUID(p.Species.ID), nullIfEmpty(p.Species.FreeText)
	}
	if p.LocationID != nil {
		params.SetLocation, params.LocationID = true, nullUUID(*p.LocationID)
	}
	if p.AcquiredOn != nil {
		params.SetAcquiredOn, params.AcquiredOn = true, pgDate(*p.AcquiredOn)
	}
	var out domain.Plant
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		if _, err := q.UpdatePlant(ctx, params); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return plantWriteError("update plant", err)
		}
		ch.add(householdID, domain.ChangePlant, id, domain.ChangeUpdated, id)
		var err error
		out, err = getPlant(ctx, q, householdID, id)
		return err
	})
	return out, err
}

// Delete は、世帯の植物を削除する。タスク・記録・メモ・提案は、外部キーの ON DELETE CASCADE でいっしょに消える。
// 変更イベントは植物の削除の1件だけにし、いっしょに消えたデータは、アプリが植物のイベントから読み取る。
func (r *PlantRepository) Delete(ctx context.Context, householdID, id uuid.UUID) error {
	return withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		n, err := q.DeletePlant(ctx, sqlcgen.DeletePlantParams{ID: id, HouseholdID: householdID})
		if err != nil {
			return fmt.Errorf("delete plant: %w", err)
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		ch.add(householdID, domain.ChangePlant, id, domain.ChangeDeleted, id)
		return nil
	})
}

// SetArchived は、植物をアーカイブする（archived が false なら元に戻す）。アーカイブし直しても、最初の日時を残す。
func (r *PlantRepository) SetArchived(ctx context.Context, householdID, id uuid.UUID, archived bool) (domain.Plant, error) {
	var out domain.Plant
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		_, err := q.SetPlantArchived(ctx, sqlcgen.SetPlantArchivedParams{ID: id, HouseholdID: householdID, Archived: archived})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("set plant archived: %w", err)
		}
		ch.add(householdID, domain.ChangePlant, id, domain.ChangeUpdated, id)
		out, err = getPlant(ctx, q, householdID, id)
		return err
	})
	return out, err
}

// plantWriteError は、種類・置き場所の外部キーの違反を、入力の誤りに変換する。
// アプリで存在を確かめてから書き込むと、そのあいだに消される場合があるため、DB の制約で判定する。
func plantWriteError(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		switch pgErr.ConstraintName {
		case plantsSpeciesFKey:
			return &domain.ValidationError{Fields: map[string]string{"speciesId": "invalid"}}
		case plantsLocationFKey:
			return &domain.ValidationError{Fields: map[string]string{"locationId": "invalid"}}
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

func toDomainPlant(row sqlcgen.Plant, speciesName *string, nextDueOn pgtype.Date) domain.Plant {
	return domain.Plant{
		ID:          row.ID,
		HouseholdID: row.HouseholdID,
		PlantProfile: domain.PlantProfile{
			Name:       row.Name,
			Species:    domain.PlantSpecies{ID: row.SpeciesID.UUID, FreeText: emptyIfNull(row.SpeciesFreeText)},
			LocationID: row.LocationID.UUID,
			AcquiredOn: civilDate(row.AcquiredOn),
		},
		SpeciesName: emptyIfNull(speciesName),
		NextDueOn:   civilDate(nextDueOn),
		ArchivedAt:  row.ArchivedAt,
		CreatedAt:   row.CreatedAt,
	}
}

// nullUUID は、uuid.Nil を NULL（未設定）にする。
func nullUUID(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: id != uuid.Nil}
}

// pgDate は、ゼロ値の日付を NULL（未設定）にする。
func pgDate(d civil.Date) pgtype.Date {
	if d.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: d.In(time.UTC), Valid: true}
}

// civilDate は、NULL の日付をゼロ値（未設定）にする。
func civilDate(d pgtype.Date) civil.Date {
	if !d.Valid {
		return civil.Date{}
	}
	return civil.DateOf(d.Time)
}

// ActiveIDs は、すべての世帯の、アーカイブしていない植物の ID を、ID の順に返す（日次の助言のジョブで使う）。
func (r *PlantRepository) ActiveIDs(ctx context.Context) ([]uuid.UUID, error) {
	ids, err := r.q.ListActivePlantIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active plant ids: %w", err)
	}
	return ids, nil
}
