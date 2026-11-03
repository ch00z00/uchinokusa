package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// InvitationService は、招待の発行・一覧・取り消しを行う（実装は app.InvitationService）。
type InvitationService interface {
	Create(ctx context.Context, actorID, householdID uuid.UUID, role string) (app.CreatedInvitation, error)
	List(ctx context.Context, actorID, householdID uuid.UUID) ([]domain.Invitation, error)
	Revoke(ctx context.Context, actorID, householdID, invitationID uuid.UUID) error
}

func (s *server) CreateInvitation(ctx context.Context, req oapi.CreateInvitationRequestObject) (oapi.CreateInvitationResponseObject, error) {
	inv, err := s.invitations.Create(ctx, currentUser(ctx).ID, req.HouseholdId, req.Body.Role)
	if err != nil {
		return nil, err
	}
	i := toInvitation(inv.Invitation)
	return oapi.CreateInvitation201JSONResponse{
		Id:        i.Id,
		Role:      i.Role,
		Code:      i.Code,
		ExpiresAt: i.ExpiresAt,
		CreatedAt: i.CreatedAt,
		Url:       inv.URL,
	}, nil
}

func (s *server) ListInvitations(ctx context.Context, req oapi.ListInvitationsRequestObject) (oapi.ListInvitationsResponseObject, error) {
	invs, err := s.invitations.List(ctx, currentUser(ctx).ID, req.HouseholdId)
	if err != nil {
		return nil, err
	}
	out := make([]oapi.Invitation, 0, len(invs))
	for _, inv := range invs {
		out = append(out, toInvitation(inv))
	}
	return oapi.ListInvitations200JSONResponse{Invitations: out}, nil
}

func (s *server) RevokeInvitation(ctx context.Context, req oapi.RevokeInvitationRequestObject) (oapi.RevokeInvitationResponseObject, error) {
	if err := s.invitations.Revoke(ctx, currentUser(ctx).ID, req.HouseholdId, req.InvitationId); err != nil {
		return nil, err
	}
	return oapi.RevokeInvitation204Response{}, nil
}

func toInvitation(inv domain.Invitation) oapi.Invitation {
	return oapi.Invitation{
		Id:        inv.ID,
		Role:      oapi.Role(inv.Role),
		Code:      inv.Code,
		ExpiresAt: inv.ExpiresAt.UTC(),
		CreatedAt: inv.CreatedAt.UTC(),
	}
}
