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

// MembershipRepository は、memberships テーブルへのアクセスを提供する。
type MembershipRepository struct {
	q *sqlcgen.Queries
}

func NewMembershipRepository(pool *pgxpool.Pool) *MembershipRepository {
	return &MembershipRepository{q: sqlcgen.New(pool)}
}

// Get は、ユーザーの世帯でのメンバーシップを返す。メンバーでなければ domain.ErrNotFound を返す。
func (r *MembershipRepository) Get(ctx context.Context, householdID, userID uuid.UUID) (domain.Membership, error) {
	row, err := r.q.GetMembership(ctx, sqlcgen.GetMembershipParams{HouseholdID: householdID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Membership{}, fmt.Errorf("get membership: %w", err)
	}
	role, err := domain.ParseRole(row.Role) // DB の CHECK 制約があるため、通常は失敗しない
	if err != nil {
		return domain.Membership{}, fmt.Errorf("get membership: %w", err)
	}
	return domain.Membership{HouseholdID: row.HouseholdID, UserID: row.UserID, Role: role}, nil
}
