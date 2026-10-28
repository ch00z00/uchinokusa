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

// UserRepository は、users テーブルへのアクセスを提供する。
type UserRepository struct {
	q *sqlcgen.Queries
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{q: sqlcgen.New(pool)}
}

// GetOrCreate は、firebase_uid が一致するユーザーを返す。いなければ作る。
// 削除済みのユーザーの場合は domain.ErrNotFound を返す。
func (r *UserRepository) GetOrCreate(ctx context.Context, id domain.Identity) (domain.User, error) {
	// ほとんどのリクエストは既存のユーザーなので、まず読むだけにする（毎回の書き込みを避ける）
	row, err := r.q.GetUserByFirebaseUID(ctx, id.UID)
	if err == nil {
		return toDomainUser(row), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}

	row, err = r.q.CreateUserIfNotExists(ctx, sqlcgen.CreateUserIfNotExistsParams{
		FirebaseUID:  id.UID,
		DisplayName:  id.Name,
		AuthProvider: id.Provider,
	})
	if err == nil {
		return toDomainUser(row), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}

	// 行が返らなかった：同時に別のリクエストが作ったか、削除済みのユーザーである
	row, err = r.q.GetUserByFirebaseUID(ctx, id.UID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user after conflict: %w", err)
	}
	return toDomainUser(row), nil
}

// UpdateDisplayName は、表示名を変更する。ユーザーがいなければ domain.ErrNotFound を返す。
func (r *UserRepository) UpdateDisplayName(ctx context.Context, userID uuid.UUID, name string) (domain.User, error) {
	row, err := r.q.UpdateUserDisplayName(ctx, sqlcgen.UpdateUserDisplayNameParams{ID: userID, DisplayName: name})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("update display name: %w", err)
	}
	return toDomainUser(row), nil
}

func toDomainUser(row sqlcgen.User) domain.User {
	return domain.User{
		ID:           row.ID,
		FirebaseUID:  row.FirebaseUID,
		DisplayName:  row.DisplayName,
		AuthProvider: row.AuthProvider,
		CreatedAt:    row.CreatedAt,
	}
}
