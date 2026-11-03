package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewInvitationRepository(pool *pgxpool.Pool) *InvitationRepository {
	return &InvitationRepository{pool: pool, q: sqlcgen.New(pool)}
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

// Preview は、有効な招待の内容と、userID がすでにメンバーかを返す。無効なら domain.ErrInvitationInvalid を返す。
func (r *InvitationRepository) Preview(ctx context.Context, key domain.InvitationKey, userID uuid.UUID) (domain.InvitationPreview, error) {
	row, err := r.q.PreviewInvitation(ctx, sqlcgen.PreviewInvitationParams{UserID: userID, Code: nullIfEmpty(key.Code), TokenHash: key.TokenHash})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.InvitationPreview{}, domain.ErrInvitationInvalid
	}
	if err != nil {
		return domain.InvitationPreview{}, fmt.Errorf("preview invitation: %w", err)
	}
	role, err := domain.ParseRole(row.Role)
	if err != nil {
		return domain.InvitationPreview{}, fmt.Errorf("preview invitation: %w", err)
	}
	return domain.InvitationPreview{HouseholdName: row.HouseholdName, Role: role, ExpiresAt: row.ExpiresAt, AlreadyMember: row.AlreadyMember}, nil
}

// memberJoinedPayload は、参加の通知のアウトボックスに入れる内容である。
type memberJoinedPayload struct {
	InvitationID uuid.UUID   `json:"invitationId"`
	Role         domain.Role `json:"role"`
}

// Accept は、userID を招待のロールで世帯に追加し、参加の通知をアウトボックスに積む。
// 招待のロック・メンバーの追加・通知の予約を1つのトランザクションで行い、どれか1つだけが起きることを防ぐ。
func (r *InvitationRepository) Accept(ctx context.Context, key domain.InvitationKey, userID uuid.UUID) (domain.HouseholdWithRole, error) {
	var out domain.HouseholdWithRole
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		inv, err := q.LockValidInvitation(ctx, sqlcgen.LockValidInvitationParams{Code: nullIfEmpty(key.Code), TokenHash: key.TokenHash})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvitationInvalid
		}
		if err != nil {
			return fmt.Errorf("lock invitation: %w", err)
		}
		role, err := domain.ParseRole(inv.Role)
		if err != nil {
			return err
		}

		_, err = q.AddMemberIfAbsent(ctx, sqlcgen.AddMemberIfAbsentParams{HouseholdID: inv.HouseholdID, UserID: userID, Role: inv.Role})
		if errors.Is(err, pgx.ErrNoRows) {
			// 同じ人の参加が同時に届いた場合も、後のほうは先のコミットを待ってからここに来る
			return &domain.ConflictError{Code: domain.ConflictAlreadyMember}
		}
		if err != nil {
			return fmt.Errorf("add member: %w", err)
		}

		payload, err := json.Marshal(memberJoinedPayload{InvitationID: inv.ID, Role: role})
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		if err := q.EnqueueMemberJoined(ctx, sqlcgen.EnqueueMemberJoinedParams{
			HouseholdID: inv.HouseholdID,
			ActorUserID: uuid.NullUUID{UUID: userID, Valid: true},
			Payload:     payload,
		}); err != nil {
			return fmt.Errorf("enqueue member joined: %w", err)
		}

		h, err := q.GetHousehold(ctx, inv.HouseholdID)
		if err != nil {
			return fmt.Errorf("get household: %w", err)
		}
		out = domain.HouseholdWithRole{Household: toDomainHousehold(h), Role: role}
		return nil
	})
	return out, err
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
