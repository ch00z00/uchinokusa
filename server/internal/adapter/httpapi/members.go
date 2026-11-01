package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// MembershipService は、メンバーの管理と通知設定を行う（実装は app.MembershipService）。
type MembershipService interface {
	List(ctx context.Context, actorID, householdID uuid.UUID) ([]domain.Member, error)
	UpdateRole(ctx context.Context, actorID, householdID, targetID uuid.UUID, role string) (domain.Member, error)
	Remove(ctx context.Context, actorID, householdID, targetID uuid.UUID) error
	TransferOwnership(ctx context.Context, actorID, householdID, targetID uuid.UUID) error
	GetNotificationSettings(ctx context.Context, actorID, householdID uuid.UUID) (domain.NotificationSettings, error)
	UpdateNotificationSettings(ctx context.Context, actorID, householdID uuid.UUID, reminderEnabled bool, reminderTime string, completionNotifyEnabled bool) (domain.NotificationSettings, error)
}

func (s *server) ListMembers(ctx context.Context, req oapi.ListMembersRequestObject) (oapi.ListMembersResponseObject, error) {
	ms, err := s.members.List(ctx, currentUser(ctx).ID, req.HouseholdId)
	if err != nil {
		return nil, err
	}
	out := make([]oapi.Member, 0, len(ms))
	for _, m := range ms {
		out = append(out, toMember(m))
	}
	return oapi.ListMembers200JSONResponse{Members: out}, nil
}

func (s *server) UpdateMemberRole(ctx context.Context, req oapi.UpdateMemberRoleRequestObject) (oapi.UpdateMemberRoleResponseObject, error) {
	m, err := s.members.UpdateRole(ctx, currentUser(ctx).ID, req.HouseholdId, req.UserId, req.Body.Role)
	if err != nil {
		return nil, err
	}
	return oapi.UpdateMemberRole200JSONResponse(toMember(m)), nil
}

func (s *server) RemoveMember(ctx context.Context, req oapi.RemoveMemberRequestObject) (oapi.RemoveMemberResponseObject, error) {
	if err := s.members.Remove(ctx, currentUser(ctx).ID, req.HouseholdId, req.UserId); err != nil {
		return nil, err
	}
	return oapi.RemoveMember204Response{}, nil
}

func (s *server) TransferOwnership(ctx context.Context, req oapi.TransferOwnershipRequestObject) (oapi.TransferOwnershipResponseObject, error) {
	if err := s.members.TransferOwnership(ctx, currentUser(ctx).ID, req.HouseholdId, req.Body.UserId); err != nil {
		return nil, err
	}
	return oapi.TransferOwnership204Response{}, nil
}

func (s *server) GetNotificationSettings(ctx context.Context, req oapi.GetNotificationSettingsRequestObject) (oapi.GetNotificationSettingsResponseObject, error) {
	ns, err := s.members.GetNotificationSettings(ctx, currentUser(ctx).ID, req.HouseholdId)
	if err != nil {
		return nil, err
	}
	return oapi.GetNotificationSettings200JSONResponse(toNotificationSettings(ns)), nil
}

func (s *server) UpdateNotificationSettings(ctx context.Context, req oapi.UpdateNotificationSettingsRequestObject) (oapi.UpdateNotificationSettingsResponseObject, error) {
	b := req.Body
	ns, err := s.members.UpdateNotificationSettings(ctx, currentUser(ctx).ID, req.HouseholdId, b.ReminderEnabled, b.ReminderTime, b.CompletionNotifyEnabled)
	if err != nil {
		return nil, err
	}
	return oapi.UpdateNotificationSettings200JSONResponse(toNotificationSettings(ns)), nil
}

func toMember(m domain.Member) oapi.Member {
	return oapi.Member{UserId: m.UserID, DisplayName: m.DisplayName, Role: oapi.Role(m.Role), JoinedAt: m.JoinedAt.UTC()}
}

func toNotificationSettings(ns domain.NotificationSettings) oapi.NotificationSettings {
	return oapi.NotificationSettings{
		ReminderEnabled:         ns.ReminderEnabled,
		ReminderTime:            ns.ReminderTimeString(),
		CompletionNotifyEnabled: ns.CompletionNotifyEnabled,
	}
}
