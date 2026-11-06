package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// MemberRepository は、世帯のメンバー（memberships テーブル）の管理を提供する。
type MemberRepository struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func NewMemberRepository(pool *pgxpool.Pool) *MemberRepository {
	return &MemberRepository{pool: pool, q: sqlcgen.New(pool)}
}

// List は、世帯のメンバーを参加の古い順に返す。
func (r *MemberRepository) List(ctx context.Context, householdID uuid.UUID) ([]domain.Member, error) {
	rows, err := r.q.ListMembers(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	out := make([]domain.Member, 0, len(rows))
	for _, row := range rows {
		m, err := toDomainMember(row.UserID, row.DisplayName, row.Role, row.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("list members: %w", err)
		}
		out = append(out, m)
	}
	return out, nil
}

// UpdateRole は、メンバーのロールを変える。オーナーの行は変えない。
func (r *MemberRepository) UpdateRole(ctx context.Context, householdID, userID uuid.UUID, role domain.Role) (domain.Member, error) {
	var out domain.Member
	err := withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		row, err := q.UpdateMemberRole(ctx, sqlcgen.UpdateMemberRoleParams{HouseholdID: householdID, UserID: userID, Role: string(role)})
		if errors.Is(err, pgx.ErrNoRows) {
			return whyUnchanged(ctx, q, householdID, userID)
		}
		if err != nil {
			return fmt.Errorf("update member role: %w", err)
		}
		ch.add(householdID, domain.ChangeMember, userID, domain.ChangeUpdated, uuid.Nil)
		out, err = toDomainMember(row.UserID, row.DisplayName, row.Role, row.CreatedAt)
		return err
	})
	return out, err
}

// Remove は、メンバーを世帯から外す。オーナーは外さない。
// 記録の作業者は users を参照し、名前も記録に持っているため、メンバーシップを消しても記録は残る（要件4-7）。
func (r *MemberRepository) Remove(ctx context.Context, householdID, userID uuid.UUID) error {
	return withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		n, err := q.RemoveMember(ctx, sqlcgen.RemoveMemberParams{HouseholdID: householdID, UserID: userID})
		if err != nil {
			return fmt.Errorf("remove member: %w", err)
		}
		if n == 0 {
			return whyUnchanged(ctx, q, householdID, userID)
		}
		// 外されたメンバーにも届くので、アプリはこの世帯の表示をやめられる（購読の見直しは 11.2）
		ch.add(householdID, domain.ChangeMember, userID, domain.ChangeDeleted, uuid.Nil)
		return nil
	})
}

// whyUnchanged は、オーナーを除く条件の更新・削除が0行だった理由を返す。
// オーナーなら last_owner の衝突、メンバーでなければ domain.ErrNotFound にする。
func whyUnchanged(ctx context.Context, q *sqlcgen.Queries, householdID, userID uuid.UUID) error {
	m, err := q.GetMembership(ctx, sqlcgen.GetMembershipParams{HouseholdID: householdID, UserID: userID})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.ErrNotFound
	case err != nil:
		return fmt.Errorf("get membership: %w", err)
	case m.Role == string(domain.RoleOwner):
		return &domain.ConflictError{Code: domain.ConflictLastOwner}
	}
	return fmt.Errorf("membership %s was not changed for an unknown reason", m.ID)
}

// TransferOwnership は、from のオーナー権限を to に移し、from を編集者にする（要件4-8）。
// 2行をトランザクションの中でロックしてから確かめるので、同時に複数の移譲が届いても、オーナーは1人のままになる。
func (r *MemberRepository) TransferOwnership(ctx context.Context, householdID, from, to uuid.UUID) error {
	return withChanges(ctx, r.pool, func(q *sqlcgen.Queries, ch *changes) error {
		// ロックを取るまで待つ。ほかの移譲が先に終わっていれば、待ったあとに最新の行が返る
		rows, err := q.LockMemberships(ctx, sqlcgen.LockMembershipsParams{HouseholdID: householdID, UserIds: []uuid.UUID{from, to}})
		if err != nil {
			return fmt.Errorf("lock memberships: %w", err)
		}
		var fromRole string
		var toFound bool
		for _, m := range rows {
			switch m.UserID {
			case from:
				fromRole = m.Role
			case to:
				toFound = true
			}
		}
		if fromRole != string(domain.RoleOwner) {
			return domain.ErrForbidden
		}
		if !toFound {
			return domain.ErrNotFound
		}
		if err := q.SetMemberRole(ctx, sqlcgen.SetMemberRoleParams{HouseholdID: householdID, UserID: from, Role: string(domain.RoleEditor)}); err != nil {
			return fmt.Errorf("demote owner: %w", err)
		}
		if err := q.SetMemberRole(ctx, sqlcgen.SetMemberRoleParams{HouseholdID: householdID, UserID: to, Role: string(domain.RoleOwner)}); err != nil {
			return fmt.Errorf("promote new owner: %w", err)
		}
		ch.add(householdID, domain.ChangeMember, from, domain.ChangeUpdated, uuid.Nil)
		ch.add(householdID, domain.ChangeMember, to, domain.ChangeUpdated, uuid.Nil)
		return nil // 関数が nil を返すとコミット、エラーを返すとロールバックする
	})
}

// GetNotificationSettings は、世帯でのユーザーの通知設定を返す。メンバーでなければ domain.ErrNotFound を返す。
func (r *MemberRepository) GetNotificationSettings(ctx context.Context, householdID, userID uuid.UUID) (domain.NotificationSettings, error) {
	row, err := r.q.GetNotificationSettings(ctx, sqlcgen.GetNotificationSettingsParams{HouseholdID: householdID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotificationSettings{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.NotificationSettings{}, fmt.Errorf("get notification settings: %w", err)
	}
	return toNotificationSettings(row.ReminderEnabled, row.ReminderTime, row.CompletionNotifyEnabled), nil
}

// UpdateNotificationSettings は、世帯でのユーザーの通知設定を変える。メンバーでなければ domain.ErrNotFound を返す。
func (r *MemberRepository) UpdateNotificationSettings(ctx context.Context, householdID, userID uuid.UUID, s domain.NotificationSettings) (domain.NotificationSettings, error) {
	row, err := r.q.UpdateNotificationSettings(ctx, sqlcgen.UpdateNotificationSettingsParams{
		HouseholdID:             householdID,
		UserID:                  userID,
		ReminderEnabled:         s.ReminderEnabled,
		ReminderTime:            toPgTime(s.ReminderTime),
		CompletionNotifyEnabled: s.CompletionNotifyEnabled,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotificationSettings{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.NotificationSettings{}, fmt.Errorf("update notification settings: %w", err)
	}
	return toNotificationSettings(row.ReminderEnabled, row.ReminderTime, row.CompletionNotifyEnabled), nil
}

func toDomainMember(userID uuid.UUID, displayName, role string, joinedAt time.Time) (domain.Member, error) {
	r, err := domain.ParseRole(role)
	if err != nil {
		return domain.Member{}, err
	}
	return domain.Member{UserID: userID, DisplayName: displayName, Role: r, JoinedAt: joinedAt}, nil
}

// pgtype.Time は、0時からのマイクロ秒で時刻を表す。

func toPgTime(t civil.Time) pgtype.Time {
	d := time.Duration(t.Hour)*time.Hour + time.Duration(t.Minute)*time.Minute
	return pgtype.Time{Microseconds: d.Microseconds(), Valid: true}
}

func toNotificationSettings(reminderEnabled bool, reminderTime pgtype.Time, completionNotifyEnabled bool) domain.NotificationSettings {
	d := time.Duration(reminderTime.Microseconds) * time.Microsecond
	return domain.NotificationSettings{
		ReminderEnabled:         reminderEnabled,
		ReminderTime:            civil.Time{Hour: int(d / time.Hour), Minute: int(d % time.Hour / time.Minute)},
		CompletionNotifyEnabled: completionNotifyEnabled,
	}
}
