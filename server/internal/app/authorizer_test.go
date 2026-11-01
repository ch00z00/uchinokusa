package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeMemberships は、app.MembershipRepository の偽物である。登録したメンバーシップだけを返す。
type fakeMemberships struct {
	byKey map[[2]uuid.UUID]domain.Membership
	err   error
}

func (f *fakeMemberships) Get(_ context.Context, householdID, userID uuid.UUID) (domain.Membership, error) {
	if f.err != nil {
		return domain.Membership{}, f.err
	}
	m, ok := f.byKey[[2]uuid.UUID{householdID, userID}]
	if !ok {
		return domain.Membership{}, domain.ErrNotFound
	}
	return m, nil
}

func TestAuthorizer_Require(t *testing.T) {
	t.Parallel()

	household := uuid.Must(uuid.NewV7())
	otherHousehold := uuid.Must(uuid.NewV7())
	viewer := uuid.Must(uuid.NewV7())
	stranger := uuid.Must(uuid.NewV7())
	viewerMembership := domain.Membership{HouseholdID: household, UserID: viewer, Role: domain.RoleViewer}
	memberships := &fakeMemberships{byKey: map[[2]uuid.UUID]domain.Membership{
		{household, viewer}: viewerMembership,
	}}

	tests := []struct {
		name        string
		userID      uuid.UUID
		householdID uuid.UUID
		action      domain.Action
		wantErr     error
	}{
		{name: "ロールが許可する操作", userID: viewer, householdID: household, action: domain.ActionRead},
		{name: "ロールが足りない操作は、権限がない（403）", userID: viewer, householdID: household, action: domain.ActionPlantWrite, wantErr: domain.ErrForbidden},
		// メンバーでない場合は、閲覧でも、どの操作でも「見つからない」にして、世帯の存在を明かさない（要件4-4）
		{name: "メンバーでない世帯の閲覧は、見つからない（404）", userID: stranger, householdID: household, action: domain.ActionRead, wantErr: domain.ErrNotFound},
		{name: "メンバーでない世帯の管理も、403 ではなく 404", userID: stranger, householdID: household, action: domain.ActionHouseholdManage, wantErr: domain.ErrNotFound},
		{name: "ほかの世帯のメンバーでも、この世帯では見つからない", userID: viewer, householdID: otherHousehold, action: domain.ActionRead, wantErr: domain.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m, err := app.NewAuthorizer(memberships).Require(t.Context(), tt.userID, tt.householdID, tt.action)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && m != viewerMembership {
				t.Errorf("membership = %+v, want %+v", m, viewerMembership)
			}
		})
	}
}

// DB の障害は、そのまま返す（403・404 にすり替えない）。
func TestAuthorizer_Require_RepositoryError(t *testing.T) {
	t.Parallel()
	dbErr := errors.New("connection refused")

	_, err := app.NewAuthorizer(&fakeMemberships{err: dbErr}).Require(t.Context(), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), domain.ActionRead)
	if !errors.Is(err, dbErr) || errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
		t.Errorf("err = %v, want the repository error", err)
	}
}
