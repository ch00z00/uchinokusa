package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

const (
	// pgUniqueViolation は、ユニーク制約の違反を表す PostgreSQL のエラーコードである。
	pgUniqueViolation = "23505"
	// invitationsCodeKey は、invitations.code のユニーク制約の名前（PostgreSQL が付けた名前）である。
	invitationsCodeKey = "invitations_code_key"
)

// InvitationRepository は、invitations テーブルへのアクセスを提供する。
type InvitationRepository struct {
	q *sqlcgen.Queries
}

func NewInvitationRepository(pool *pgxpool.Pool) *InvitationRepository {
	return &InvitationRepository{q: sqlcgen.New(pool)}
}

// Create は、招待を保存する。トークンはハッシュだけを保存する。
// コードがほかの招待と重なれば app.ErrInvitationCodeTaken を返す。
func (r *InvitationRepository) Create(ctx context.Context, householdID, createdBy uuid.UUID, role domain.Role, s domain.InvitationSecret) (domain.Invitation, error) {
	row, err := r.q.CreateInvitation(ctx, sqlcgen.CreateInvitationParams{
		HouseholdID: householdID,
		Role:        string(role),
		TokenHash:   s.TokenHash,
		Code:        s.Code,
		CreatedBy:   uuid.NullUUID{UUID: createdBy, Valid: true},
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == invitationsCodeKey {
		return domain.Invitation{}, app.ErrInvitationCodeTaken
	}
	if err != nil {
		return domain.Invitation{}, fmt.Errorf("create invitation: %w", err)
	}
	return toDomainInvitation(row)
}

// ListActive は、取り消されておらず期限内の招待を、新しい順に返す。
func (r *InvitationRepository) ListActive(ctx context.Context, householdID uuid.UUID) ([]domain.Invitation, error) {
	rows, err := r.q.ListActiveInvitations(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	out := make([]domain.Invitation, 0, len(rows))
	for _, row := range rows {
		inv, err := toDomainInvitation(row)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, nil
}

// Revoke は、招待を取り消す。世帯の招待でなければ domain.ErrNotFound を返す。取り消し済みでもエラーにしない。
func (r *InvitationRepository) Revoke(ctx context.Context, householdID, invitationID uuid.UUID) error {
	n, err := r.q.RevokeInvitation(ctx, sqlcgen.RevokeInvitationParams{HouseholdID: householdID, ID: invitationID})
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func toDomainInvitation(row sqlcgen.Invitation) (domain.Invitation, error) {
	role, err := domain.ParseRole(row.Role)
	if err != nil {
		return domain.Invitation{}, fmt.Errorf("invitation %s: %w", row.ID, err)
	}
	return domain.Invitation{
		ID:          row.ID,
		HouseholdID: row.HouseholdID,
		Role:        role,
		Code:        row.Code,
		ExpiresAt:   row.ExpiresAt,
		CreatedAt:   row.CreatedAt,
	}, nil
}
