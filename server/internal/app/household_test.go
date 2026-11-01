package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeHouseholds は、app.HouseholdRepository の偽物である。呼ばれた操作を記録する。
type fakeHouseholds struct {
	created []domain.NewHousehold
	patched []domain.HouseholdPatch
	deleted []uuid.UUID
}

func (f *fakeHouseholds) CreateWithOwner(_ context.Context, _ uuid.UUID, h domain.NewHousehold) (domain.Household, error) {
	f.created = append(f.created, h)
	return domain.Household{ID: uuid.Must(uuid.NewV7()), Name: h.Name, Region: h.Region}, nil
}

func (f *fakeHouseholds) ListByUser(_ context.Context, _ uuid.UUID) ([]domain.HouseholdWithRole, error) {
	return []domain.HouseholdWithRole{}, nil
}

func (f *fakeHouseholds) Update(_ context.Context, id uuid.UUID, p domain.HouseholdPatch) (domain.Household, error) {
	f.patched = append(f.patched, p)
	return domain.Household{ID: id}, nil
}

func (f *fakeHouseholds) Delete(_ context.Context, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	return nil
}

// householdFixture は、1つの世帯にオーナー・編集者・閲覧者がいる状態を用意する。
type householdFixture struct {
	household, owner, editor, viewer, stranger uuid.UUID
	households                                 *fakeHouseholds
	svc                                        *app.HouseholdService
}

func newHouseholdFixture() householdFixture {
	f := householdFixture{
		household:  uuid.Must(uuid.NewV7()),
		owner:      uuid.Must(uuid.NewV7()),
		editor:     uuid.Must(uuid.NewV7()),
		viewer:     uuid.Must(uuid.NewV7()),
		stranger:   uuid.Must(uuid.NewV7()),
		households: &fakeHouseholds{},
	}
	memberships := &fakeMemberships{byKey: map[[2]uuid.UUID]domain.Membership{}}
	for user, role := range map[uuid.UUID]domain.Role{f.owner: domain.RoleOwner, f.editor: domain.RoleEditor, f.viewer: domain.RoleViewer} {
		memberships.byKey[[2]uuid.UUID{f.household, user}] = domain.Membership{HouseholdID: f.household, UserID: user, Role: role}
	}
	f.svc = app.NewHouseholdService(f.households, app.NewAuthorizer(memberships))
	return f
}

func TestHouseholdService_Create(t *testing.T) {
	t.Parallel()

	t.Run("作成したユーザーはオーナーになる", func(t *testing.T) {
		t.Parallel()
		f := newHouseholdFixture()
		h, err := f.svc.Create(t.Context(), f.stranger, " 自宅 ", ptr("東京都"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if h.Role != domain.RoleOwner || h.Name != "自宅" {
			t.Errorf("household = %+v, want owner of 自宅", h)
		}
		want := domain.NewHousehold{Name: "自宅", Region: domain.Region{Prefecture: "東京都"}}
		if len(f.households.created) != 1 || f.households.created[0] != want {
			t.Errorf("created = %+v, want [%+v]", f.households.created, want)
		}
	})

	t.Run("不正な内容は保存しない", func(t *testing.T) {
		t.Parallel()
		f := newHouseholdFixture()
		_, err := f.svc.Create(t.Context(), f.stranger, "", nil, nil)
		var verr *domain.ValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("err = %v, want *ValidationError", err)
		}
		if len(f.households.created) != 0 {
			t.Error("repository was called")
		}
	})
}

func TestHouseholdService_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		user    func(householdFixture) uuid.UUID
		newName string
		wantErr error
	}{
		{name: "オーナーは変更できる", user: func(f householdFixture) uuid.UUID { return f.owner }, newName: "自宅2"},
		{name: "編集者は変更できない", user: func(f householdFixture) uuid.UUID { return f.editor }, newName: "自宅2", wantErr: domain.ErrForbidden},
		{name: "閲覧者は変更できない", user: func(f householdFixture) uuid.UUID { return f.viewer }, newName: "自宅2", wantErr: domain.ErrForbidden},
		{name: "メンバーでなければ、見つからない", user: func(f householdFixture) uuid.UUID { return f.stranger }, newName: "自宅2", wantErr: domain.ErrNotFound},
		// 権限の判定を入力の検証より先に行う。メンバーでない人に 422 を返すと、世帯の存在がわかってしまう
		{name: "メンバーでなければ、不正な入力でも見つからない", user: func(f householdFixture) uuid.UUID { return f.stranger }, newName: "", wantErr: domain.ErrNotFound},
		{name: "編集者の不正な入力は、権限がない", user: func(f householdFixture) uuid.UUID { return f.editor }, newName: "", wantErr: domain.ErrForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newHouseholdFixture()
			h, err := f.svc.Update(t.Context(), tt.user(f), f.household, ptr(tt.newName), nil, nil)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if len(f.households.patched) != 0 {
					t.Error("repository was called")
				}
				return
			}
			if h.Role != domain.RoleOwner {
				t.Errorf("role = %s, want owner", h.Role)
			}
			if len(f.households.patched) != 1 || *f.households.patched[0].Name != tt.newName {
				t.Errorf("patched = %+v", f.households.patched)
			}
		})
	}

	t.Run("オーナーの不正な入力は、検証のエラー", func(t *testing.T) {
		t.Parallel()
		f := newHouseholdFixture()
		_, err := f.svc.Update(t.Context(), f.owner, f.household, ptr(""), nil, nil)
		var verr *domain.ValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("err = %v, want *ValidationError", err)
		}
	})
}

func TestHouseholdService_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		user    func(householdFixture) uuid.UUID
		wantErr error
	}{
		{name: "オーナーは削除できる", user: func(f householdFixture) uuid.UUID { return f.owner }},
		{name: "編集者は削除できない", user: func(f householdFixture) uuid.UUID { return f.editor }, wantErr: domain.ErrForbidden},
		{name: "閲覧者は削除できない", user: func(f householdFixture) uuid.UUID { return f.viewer }, wantErr: domain.ErrForbidden},
		{name: "メンバーでなければ、見つからない", user: func(f householdFixture) uuid.UUID { return f.stranger }, wantErr: domain.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newHouseholdFixture()
			err := f.svc.Delete(t.Context(), tt.user(f), f.household)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			wantDeleted := 0
			if tt.wantErr == nil {
				wantDeleted = 1
			}
			if len(f.households.deleted) != wantDeleted {
				t.Errorf("deleted = %v, want %d call(s)", f.households.deleted, wantDeleted)
			}
		})
	}
}

func ptr(s string) *string { return &s }
