package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// LocationRepository は、locations テーブルへのアクセスを提供する。
type LocationRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewLocationRepository(pool *pgxpool.Pool) *LocationRepository {
	return &LocationRepository{pool: pool, q: sqlcgen.New(pool)}
}

// List は、世帯のすべての置き場所を、作成の古い順に、植物（アーカイブしたものを除く）の数とともに返す。
func (r *LocationRepository) List(ctx context.Context, householdID uuid.UUID) ([]domain.Location, error) {
	rows, err := r.q.ListLocations(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	out := make([]domain.Location, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainLocation(row.Location, row.PlantCount))
	}
	return out, nil
}

// Create は、世帯に置き場所を作る。環境の項目の空文字は、NULL（未設定）にする。
func (r *LocationRepository) Create(ctx context.Context, householdID uuid.UUID, p domain.LocationProfile) (domain.Location, error) {
	var out domain.Location
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		row, err := q.CreateLocation(ctx, sqlcgen.CreateLocationParams{
			HouseholdID:     householdID,
			Name:            p.Name,
			IsIndoor:        p.IsIndoor,
			WindowDirection: nullIfEmpty(string(p.WindowDirection)),
			WindowDistance:  nullIfEmpty(string(p.WindowDistance)),
			Shading:         nullIfEmpty(string(p.Shading)),
		})
		if err != nil {
			return fmt.Errorf("create location: %w", err)
		}
		out = toDomainLocation(row, 0)
		ch.add(householdID, domain.ChangeLocation, out.ID, domain.ChangeCreated, uuid.Nil)
		return nil
	})
	return out, err
}

// HouseholdOf は、置き場所が属する世帯の ID を返す。置き場所がなければ domain.ErrNotFound を返す。
func (r *LocationRepository) HouseholdOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	householdID, err := r.q.GetLocationHouseholdID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get location: %w", err)
	}
	return householdID, nil
}

// Update は、世帯の置き場所を変更する。世帯の置き場所でなければ domain.ErrNotFound を返す。
func (r *LocationRepository) Update(ctx context.Context, householdID, id uuid.UUID, p domain.LocationPatch) (domain.Location, error) {
	params := sqlcgen.UpdateLocationParams{ID: id, HouseholdID: householdID, Name: p.Name, IsIndoor: p.IsIndoor}
	if p.WindowDirection != nil {
		params.SetWindowDirection, params.WindowDirection = true, nullIfEmpty(string(*p.WindowDirection))
	}
	if p.WindowDistance != nil {
		params.SetWindowDistance, params.WindowDistance = true, nullIfEmpty(string(*p.WindowDistance))
	}
	if p.Shading != nil {
		params.SetShading, params.Shading = true, nullIfEmpty(string(*p.Shading))
	}
	var out domain.Location
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		row, err := q.UpdateLocation(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("update location: %w", err)
		}
		out = toDomainLocation(sqlcgen.Location{
			ID: row.ID, HouseholdID: row.HouseholdID, Name: row.Name, IsIndoor: row.IsIndoor,
			WindowDirection: row.WindowDirection, WindowDistance: row.WindowDistance, Shading: row.Shading,
			Attributes: row.Attributes, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}, row.PlantCount)
		ch.add(householdID, domain.ChangeLocation, id, domain.ChangeUpdated, uuid.Nil)
		return nil
	})
	return out, err
}

// Delete は、世帯の置き場所を削除する。紐づく植物の location_id は、外部キーの ON DELETE SET NULL で NULL になる。
// 変更イベントは置き場所の削除の1件だけにし、植物の変更はアプリが置き場所のイベントから読み取る。
func (r *LocationRepository) Delete(ctx context.Context, householdID, id uuid.UUID) error {
	return withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		n, err := q.DeleteLocation(ctx, sqlcgen.DeleteLocationParams{ID: id, HouseholdID: householdID})
		if err != nil {
			return fmt.Errorf("delete location: %w", err)
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		ch.add(householdID, domain.ChangeLocation, id, domain.ChangeDeleted, uuid.Nil)
		return nil
	})
}

func toDomainLocation(row sqlcgen.Location, plantCount int64) domain.Location {
	return domain.Location{
		ID:          row.ID,
		HouseholdID: row.HouseholdID,
		LocationProfile: domain.LocationProfile{
			Name:            row.Name,
			IsIndoor:        row.IsIndoor,
			WindowDirection: domain.WindowDirection(emptyIfNull(row.WindowDirection)),
			WindowDistance:  domain.WindowDistance(emptyIfNull(row.WindowDistance)),
			Shading:         domain.Shading(emptyIfNull(row.Shading)),
		},
		PlantCount: int(plantCount),
		CreatedAt:  row.CreatedAt,
	}
}
