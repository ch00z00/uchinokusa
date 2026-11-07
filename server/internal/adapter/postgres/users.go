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
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool, q: sqlcgen.New(pool)}
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
// 所属するすべての世帯に、メンバーの変更として知らせる（メンバーの一覧に表示名が出るため）。
func (r *UserRepository) UpdateDisplayName(ctx context.Context, userID uuid.UUID, name string) (domain.User, error) {
	var out domain.User
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		row, err := q.UpdateUserDisplayName(ctx, sqlcgen.UpdateUserDisplayNameParams{ID: userID, DisplayName: name})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("update display name: %w", err)
		}
		out = toDomainUser(row)
		households, err := q.ListUserHouseholdIDs(ctx, userID)
		if err != nil {
			return fmt.Errorf("list user households: %w", err)
		}
		for _, h := range households {
			ch.add(h, domain.ChangeMember, userID, domain.ChangeUpdated, uuid.Nil)
		}
		return nil
	})
	return out, err
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

// Delete は、1つのトランザクションでアカウントを削除する（要件1-6・1-7）。
//  1. 所属する世帯の行をロックし、ほかにメンバーのいる世帯の唯一のオーナーなら、何も消さずに domain.OwnerTransferRequiredError を返す
//  2. 自分だけの世帯は関連データごと消し、ほかの世帯からは外す（世帯の削除・メンバーの削除の変更イベントを出す）
//  3. 記録・メモの作業者名を「退会したユーザー」にし、まだ送っていない通知を取り消し、端末を消す。
//     ユーザーの行は、表示名を消して削除済みの印（deleted_at）として残す（古い ID トークンで作り直させないため）
//  4. コミットの直前に beforeCommit（Apple のトークンの失効、Firebase のユーザーの削除）を呼ぶ。失敗したらロールバックして、何も消さない
//
// 世帯の行を最初にロックするので、ほかの書き込み（世帯の行を最後にロックする。docs/decisions/0029）と重なると、
// まれにデッドロックになる。PostgreSQL が片方を失敗させるので、やり直せば消せる。
func (r *UserRepository) Delete(ctx context.Context, userID uuid.UUID, beforeCommit func(context.Context, domain.User) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		user, err := q.LockUserForDeletion(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock user: %w", err)
		}
		households, err := q.LockUserHouseholds(ctx, userID)
		if err != nil {
			return fmt.Errorf("lock user households: %w", err)
		}
		ids := make([]uuid.UUID, 0, len(households))
		for _, h := range households {
			ids = append(ids, h.ID)
		}
		rows, err := q.ListHouseholdMembership(ctx, sqlcgen.ListHouseholdMembershipParams{UserID: userID, HouseholdIds: ids})
		if err != nil {
			return fmt.Errorf("list household membership: %w", err)
		}
		membership := make(map[uuid.UUID]sqlcgen.ListHouseholdMembershipRow, len(rows))
		for _, row := range rows {
			membership[row.HouseholdID] = row
		}

		var blocked []domain.Household
		var solo, shared []uuid.UUID
		for _, h := range households {
			m := membership[h.ID]
			switch {
			case m.Members <= 1:
				solo = append(solo, h.ID)
			case m.Role == string(domain.RoleOwner):
				// オーナーは常に1人なので、ほかにメンバーがいれば、自分が唯一のオーナー
				blocked = append(blocked, domain.Household{ID: h.ID, Name: h.Name})
			default:
				shared = append(shared, h.ID)
			}
		}
		if len(blocked) > 0 {
			return &domain.OwnerTransferRequiredError{Households: blocked}
		}

		// 世帯を消すとバージョンを進められないので、消す前に発行する（届くのはコミットしたあと）
		var events []domain.ChangeEvent
		for _, id := range solo {
			events = append(events, domain.ChangeEvent{HouseholdID: id, Entity: domain.ChangeHousehold, EntityID: id, Op: domain.ChangeDeleted})
		}
		for _, id := range shared {
			// 記録・メモの作業者名の匿名化は、イベントを出さない。アプリは、メンバーの削除を受けて世帯のデータを取り直す
			events = append(events, domain.ChangeEvent{HouseholdID: id, Entity: domain.ChangeMember, EntityID: userID, Op: domain.ChangeDeleted})
		}
		if err := publishChanges(ctx, q, events); err != nil {
			return err
		}
		for _, id := range solo {
			if err := q.DeleteHousehold(ctx, id); err != nil {
				return fmt.Errorf("delete household: %w", err)
			}
		}
		for _, id := range shared {
			if err := q.DeleteAccountMembership(ctx, sqlcgen.DeleteAccountMembershipParams{HouseholdID: id, UserID: userID}); err != nil {
				return fmt.Errorf("delete membership: %w", err)
			}
		}
		actor := nullUUID(userID)
		if err := q.AnonymizeCareRecords(ctx, sqlcgen.AnonymizeCareRecordsParams{Name: domain.DeletedUserName, UserID: actor}); err != nil {
			return fmt.Errorf("anonymize care records: %w", err)
		}
		if err := q.AnonymizeNotes(ctx, sqlcgen.AnonymizeNotesParams{Name: domain.DeletedUserName, UserID: actor}); err != nil {
			return fmt.Errorf("anonymize notes: %w", err)
		}
		if err := q.CancelActorNotices(ctx, actor); err != nil {
			return fmt.Errorf("cancel actor notices: %w", err)
		}
		if err := q.DeleteAllUserDevices(ctx, userID); err != nil {
			return fmt.Errorf("delete devices: %w", err)
		}
		if err := q.MarkUserDeleted(ctx, userID); err != nil {
			return fmt.Errorf("mark user deleted: %w", err)
		}
		// 取り消せない外部の処理は、データベースの処理がすべて済んでから、コミットの直前に行う
		return beforeCommit(ctx, toDomainUser(user))
	})
}
