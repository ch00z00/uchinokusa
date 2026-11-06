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

// HouseholdRepository は、households テーブルへのアクセスを提供する。
type HouseholdRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewHouseholdRepository(pool *pgxpool.Pool) *HouseholdRepository {
	return &HouseholdRepository{pool: pool, q: sqlcgen.New(pool)}
}

// CreateWithOwner は、世帯と、ownerID のユーザーのオーナーのメンバーシップを、1つの文でまとめて作る。
func (r *HouseholdRepository) CreateWithOwner(ctx context.Context, ownerID uuid.UUID, h domain.NewHousehold) (domain.Household, error) {
	var out domain.Household
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		row, err := q.CreateHouseholdWithOwner(ctx, sqlcgen.CreateHouseholdWithOwnerParams{
			Name:             h.Name,
			RegionPrefecture: nullIfEmpty(h.Region.Prefecture),
			RegionCity:       nullIfEmpty(h.Region.City),
			OwnerID:          ownerID,
		})
		if err != nil {
			return fmt.Errorf("create household: %w", err)
		}
		// CTE の結果は、sqlc では households とは別の型になる。列は同じなので型を変換する
		// （households に列を足して不一致になれば、コンパイルエラーで気づける）
		out = toDomainHousehold(sqlcgen.Household(row))
		// 作った人のほかの端末の世帯の一覧を、更新させるため
		ch.add(out.ID, domain.ChangeHousehold, out.ID, domain.ChangeCreated, uuid.Nil)
		return nil
	})
	return out, err
}

// ListByUser は、ユーザーが所属するすべての世帯を、作成の古い順に、そのユーザーのロールとともに返す。
func (r *HouseholdRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.HouseholdWithRole, error) {
	rows, err := r.q.ListHouseholdsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list households: %w", err)
	}
	out := make([]domain.HouseholdWithRole, 0, len(rows)) // 0件でも nil にしない（JSON で null ではなく [] にする）
	for _, row := range rows {
		role, err := domain.ParseRole(row.Role)
		if err != nil {
			return nil, fmt.Errorf("list households: %w", err)
		}
		out = append(out, domain.HouseholdWithRole{Household: toDomainHousehold(row.Household), Role: role})
	}
	return out, nil
}

// Update は、世帯を変更する。世帯がなければ domain.ErrNotFound を返す。
// 地域の空文字は、NULL（未設定）にする。
func (r *HouseholdRepository) Update(ctx context.Context, id uuid.UUID, p domain.HouseholdPatch) (domain.Household, error) {
	params := sqlcgen.UpdateHouseholdParams{ID: id, Name: p.Name}
	if p.Prefecture != nil {
		params.SetPrefecture, params.RegionPrefecture = true, nullIfEmpty(*p.Prefecture)
	}
	if p.City != nil {
		params.SetCity, params.RegionCity = true, nullIfEmpty(*p.City)
	}
	var out domain.Household
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		row, err := q.UpdateHousehold(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("update household: %w", err)
		}
		out = toDomainHousehold(row)
		ch.add(id, domain.ChangeHousehold, id, domain.ChangeUpdated, uuid.Nil)
		return nil
	})
	return out, err
}

// Delete は、世帯を削除する。配下のデータは ON DELETE CASCADE で消える。
// 世帯の行を消すとバージョン番号を進められないので、削除のイベントは、消す前に同じトランザクションで発行する
// （通知が届くのは、コミットしたあと）。
func (r *HouseholdRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		err := publishChanges(ctx, q, []domain.ChangeEvent{{HouseholdID: id, Entity: domain.ChangeHousehold, EntityID: id, Op: domain.ChangeDeleted}})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // すでに消えている
		}
		if err != nil {
			return err
		}
		if err := q.DeleteHousehold(ctx, id); err != nil {
			return fmt.Errorf("delete household: %w", err)
		}
		return nil
	})
}

func toDomainHousehold(row sqlcgen.Household) domain.Household {
	return domain.Household{
		ID:         row.ID,
		Name:       row.Name,
		Region:     domain.Region{Prefecture: emptyIfNull(row.RegionPrefecture), City: emptyIfNull(row.RegionCity)},
		TimeZone:   row.TimeZone,
		Hemisphere: domain.Hemisphere(row.Hemisphere),
		CreatedAt:  row.CreatedAt,
	}
}

// nullIfEmpty は、空文字を NULL（nil）にする。未設定を表す値を、DB では NULL にそろえる。
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func emptyIfNull(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
