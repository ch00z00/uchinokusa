package app

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// MemberRepository は、世帯のメンバーの保存先である（実装は adapter/postgres）。
type MemberRepository interface {
	// List は、世帯のメンバーを参加の古い順に返す。
	List(ctx context.Context, householdID uuid.UUID) ([]domain.Member, error)
	// UpdateRole は、メンバーのロールを変える。メンバーでなければ domain.ErrNotFound、
	// 対象がオーナーなら domain.ConflictError（last_owner）を返す。
	UpdateRole(ctx context.Context, householdID, userID uuid.UUID, role domain.Role) (domain.Member, error)
	// Remove は、メンバーを世帯から外す。メンバーでなければ domain.ErrNotFound、
	// 対象がオーナーなら domain.ConflictError（last_owner）を返す。
	Remove(ctx context.Context, householdID, userID uuid.UUID) error
	// TransferOwnership は、from（オーナー）のオーナー権限を to に移し、from を編集者にする。
	// from がオーナーでなければ domain.ErrForbidden、to がメンバーでなければ domain.ErrNotFound を返す。
	TransferOwnership(ctx context.Context, householdID, from, to uuid.UUID) error
	GetNotificationSettings(ctx context.Context, householdID, userID uuid.UUID) (domain.NotificationSettings, error)
	UpdateNotificationSettings(ctx context.Context, householdID, userID uuid.UUID, s domain.NotificationSettings) (domain.NotificationSettings, error)
}

// MembershipService は、メンバーの一覧・ロールの変更・削除・退出・オーナー権限の移譲・通知設定を扱う（要件2・4・15・16）。
// 世帯のオーナーは常に1人とする。オーナーの交代は移譲だけで行い、オーナーは退出もロールの変更もできない。
type MembershipService struct {
	members MemberRepository
	authz   *Authorizer
}

func NewMembershipService(members MemberRepository, authz *Authorizer) *MembershipService {
	return &MembershipService{members: members, authz: authz}
}

// List は、世帯のメンバーの一覧を返す。メンバーなら誰でも見られる。
func (s *MembershipService) List(ctx context.Context, actorID, householdID uuid.UUID) ([]domain.Member, error) {
	if _, err := s.authz.Require(ctx, actorID, householdID, domain.ActionRead); err != nil {
		return nil, err
	}
	return s.members.List(ctx, householdID)
}

// UpdateRole は、メンバーのロールを編集者・閲覧者に変える。オーナーだけができる（要件4-6）。
func (s *MembershipService) UpdateRole(ctx context.Context, actorID, householdID, targetID uuid.UUID, role string) (domain.Member, error) {
	if _, err := s.authz.Require(ctx, actorID, householdID, domain.ActionMemberManage); err != nil {
		return domain.Member{}, err
	}
	r, err := domain.ParseAssignableRole(role)
	if err != nil {
		return domain.Member{}, err
	}
	if targetID == actorID {
		// オーナーが自分を編集者・閲覧者にすると、オーナーがいなくなる
		return domain.Member{}, &domain.ConflictError{Code: domain.ConflictLastOwner}
	}
	return s.members.UpdateRole(ctx, householdID, targetID, r)
}

// Remove は、メンバーを世帯から外す。自分なら退出（オーナー以外、要件2-7・2-8）、
// ほかの人ならオーナーによる削除（要件4-7）になる。外れたメンバーの記録は、作業者名を残したまま残る。
func (s *MembershipService) Remove(ctx context.Context, actorID, householdID, targetID uuid.UUID) error {
	if targetID == actorID {
		m, err := s.authz.Require(ctx, actorID, householdID, domain.ActionRead)
		if err != nil {
			return err
		}
		if m.Role == domain.RoleOwner {
			// ほかにメンバーがいれば先に移譲し、自分だけなら世帯を削除してもらう（確認のある操作に寄せる）
			return &domain.ConflictError{Code: domain.ConflictLastOwner}
		}
		return s.members.Remove(ctx, householdID, actorID)
	}
	if _, err := s.authz.Require(ctx, actorID, householdID, domain.ActionMemberManage); err != nil {
		return err
	}
	return s.members.Remove(ctx, householdID, targetID)
}

// TransferOwnership は、オーナー権限を別のメンバーに移す。元のオーナーは編集者になる（要件4-8）。
func (s *MembershipService) TransferOwnership(ctx context.Context, actorID, householdID, targetID uuid.UUID) error {
	if _, err := s.authz.Require(ctx, actorID, householdID, domain.ActionMemberManage); err != nil {
		return err
	}
	if targetID == actorID {
		return &domain.ValidationError{Fields: map[string]string{"userId": "invalid"}}
	}
	err := s.members.TransferOwnership(ctx, householdID, actorID, targetID)
	if errors.Is(err, domain.ErrNotFound) {
		// 世帯にはアクセスできているので、404 ではなく入力の誤りとして返す
		return &domain.ValidationError{Fields: map[string]string{"userId": "not_member"}}
	}
	return err
}

// GetNotificationSettings は、世帯での自分の通知設定を返す。
func (s *MembershipService) GetNotificationSettings(ctx context.Context, actorID, householdID uuid.UUID) (domain.NotificationSettings, error) {
	if _, err := s.authz.Require(ctx, actorID, householdID, domain.ActionSettingsSelf); err != nil {
		return domain.NotificationSettings{}, err
	}
	return s.members.GetNotificationSettings(ctx, householdID, actorID)
}

// UpdateNotificationSettings は、世帯での自分の通知設定を変える。閲覧者も変えられる（要件4-1・15-3・16-3）。
func (s *MembershipService) UpdateNotificationSettings(ctx context.Context, actorID, householdID uuid.UUID, reminderEnabled bool, reminderTime string, completionNotifyEnabled bool) (domain.NotificationSettings, error) {
	if _, err := s.authz.Require(ctx, actorID, householdID, domain.ActionSettingsSelf); err != nil {
		return domain.NotificationSettings{}, err
	}
	settings, err := domain.NewNotificationSettings(reminderEnabled, reminderTime, completionNotifyEnabled)
	if err != nil {
		return domain.NotificationSettings{}, err
	}
	return s.members.UpdateNotificationSettings(ctx, householdID, actorID, settings)
}
