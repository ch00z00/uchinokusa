package app_test

import (
	"context"
	"errors"
	"maps"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeMembers は、app.MemberRepository の偽物である。呼ばれた操作を記録する。
// 世帯のメンバーは householdFixture と同じ（オーナー・編集者・閲覧者）とする。
type fakeMembers struct {
	f           *householdFixture
	roleChanges []domain.Role
	removed     []uuid.UUID
	transfers   [][2]uuid.UUID
	settings    []domain.NotificationSettings
}

func (m *fakeMembers) isMember(userID uuid.UUID) bool {
	return userID == m.f.owner || userID == m.f.editor || userID == m.f.viewer
}

func (m *fakeMembers) List(_ context.Context, _ uuid.UUID) ([]domain.Member, error) {
	return []domain.Member{}, nil
}

func (m *fakeMembers) UpdateRole(_ context.Context, _, userID uuid.UUID, role domain.Role) (domain.Member, error) {
	if !m.isMember(userID) {
		return domain.Member{}, domain.ErrNotFound
	}
	m.roleChanges = append(m.roleChanges, role)
	return domain.Member{UserID: userID, Role: role}, nil
}

func (m *fakeMembers) Remove(_ context.Context, _, userID uuid.UUID) error {
	if !m.isMember(userID) {
		return domain.ErrNotFound
	}
	m.removed = append(m.removed, userID)
	return nil
}

func (m *fakeMembers) TransferOwnership(_ context.Context, _, from, to uuid.UUID) error {
	if !m.isMember(to) {
		return domain.ErrNotFound
	}
	m.transfers = append(m.transfers, [2]uuid.UUID{from, to})
	return nil
}

func (m *fakeMembers) GetNotificationSettings(_ context.Context, _, _ uuid.UUID) (domain.NotificationSettings, error) {
	return domain.NotificationSettings{ReminderEnabled: true, ReminderTime: civil.Time{Hour: 8}, CompletionNotifyEnabled: true}, nil
}

func (m *fakeMembers) UpdateNotificationSettings(_ context.Context, _, _ uuid.UUID, s domain.NotificationSettings) (domain.NotificationSettings, error) {
	m.settings = append(m.settings, s)
	return s, nil
}

func newMembershipService() (householdFixture, *fakeMembers, *app.MembershipService) {
	f := newHouseholdFixture()
	members := &fakeMembers{f: &f}
	return f, members, app.NewMembershipService(members, f.authz)
}

type pick func(householdFixture) uuid.UUID

var (
	owner    pick = func(f householdFixture) uuid.UUID { return f.owner }
	editor   pick = func(f householdFixture) uuid.UUID { return f.editor }
	viewer   pick = func(f householdFixture) uuid.UUID { return f.viewer }
	stranger pick = func(f householdFixture) uuid.UUID { return f.stranger }
)

func lastOwner(err error) bool {
	var c *domain.ConflictError
	return errors.As(err, &c) && c.Code == domain.ConflictLastOwner
}

func TestMembershipService_UpdateRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		actor, target pick
		role          string
		wantErr       error
		wantLastOwner bool
		wantFields    map[string]string
	}{
		{name: "オーナーは、編集者を閲覧者にできる", actor: owner, target: editor, role: "viewer"},
		{name: "オーナーは、閲覧者を編集者にできる", actor: owner, target: viewer, role: "editor"},
		{name: "owner は指定できない（交代は移譲で行う）", actor: owner, target: editor, role: "owner", wantFields: map[string]string{"role": "invalid"}},
		{name: "オーナーは、自分のロールを変えられない", actor: owner, target: owner, role: "editor", wantLastOwner: true},
		{name: "編集者は、ロールを変えられない", actor: editor, target: viewer, role: "editor", wantErr: domain.ErrForbidden},
		{name: "メンバーでなければ、見つからない", actor: stranger, target: viewer, role: "editor", wantErr: domain.ErrNotFound},
		{name: "対象がメンバーでなければ、見つからない", actor: owner, target: stranger, role: "editor", wantErr: domain.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f, members, svc := newMembershipService()
			_, err := svc.UpdateRole(t.Context(), tt.actor(f), f.household, tt.target(f), tt.role)

			switch {
			case tt.wantLastOwner:
				if !lastOwner(err) {
					t.Fatalf("err = %v, want last_owner", err)
				}
			case tt.wantFields != nil:
				assertFields(t, err, tt.wantFields)
			default:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			}
			if succeeded := tt.wantErr == nil && !tt.wantLastOwner && tt.wantFields == nil; succeeded != (len(members.roleChanges) == 1) {
				t.Errorf("roleChanges = %v", members.roleChanges)
			}
		})
	}
}

func TestMembershipService_Remove(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		actor, target pick
		wantErr       error
		wantLastOwner bool
	}{
		// 退出（自分を外す）：オーナー以外は誰でもできる（要件2-7）
		{name: "編集者は退出できる", actor: editor, target: editor},
		{name: "閲覧者は退出できる", actor: viewer, target: viewer},
		{name: "オーナーは退出できない", actor: owner, target: owner, wantLastOwner: true},
		// 削除（ほかの人を外す）：オーナーだけができる
		{name: "オーナーは、編集者を外せる", actor: owner, target: editor},
		{name: "オーナーは、閲覧者を外せる", actor: owner, target: viewer},
		{name: "編集者は、ほかの人を外せない", actor: editor, target: viewer, wantErr: domain.ErrForbidden},
		{name: "閲覧者は、オーナーを外せない", actor: viewer, target: owner, wantErr: domain.ErrForbidden},
		{name: "メンバーでなければ、見つからない", actor: stranger, target: stranger, wantErr: domain.ErrNotFound},
		{name: "対象がメンバーでなければ、見つからない", actor: owner, target: stranger, wantErr: domain.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f, members, svc := newMembershipService()
			err := svc.Remove(t.Context(), tt.actor(f), f.household, tt.target(f))

			if tt.wantLastOwner {
				if !lastOwner(err) {
					t.Fatalf("err = %v, want last_owner", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if succeeded := tt.wantErr == nil && !tt.wantLastOwner; succeeded != (len(members.removed) == 1) {
				t.Errorf("removed = %v", members.removed)
			}
		})
	}
}

func TestMembershipService_TransferOwnership(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		actor, target pick
		wantErr       error
		wantFields    map[string]string
	}{
		{name: "オーナーは、編集者に移せる", actor: owner, target: editor},
		{name: "オーナーは、閲覧者に移せる", actor: owner, target: viewer},
		{name: "自分には移せない", actor: owner, target: owner, wantFields: map[string]string{"userId": "invalid"}},
		{name: "メンバーでない人には移せない", actor: owner, target: stranger, wantFields: map[string]string{"userId": "not_member"}},
		{name: "編集者は移せない", actor: editor, target: viewer, wantErr: domain.ErrForbidden},
		{name: "メンバーでなければ、見つからない", actor: stranger, target: editor, wantErr: domain.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f, members, svc := newMembershipService()
			err := svc.TransferOwnership(t.Context(), tt.actor(f), f.household, tt.target(f))

			if tt.wantFields != nil {
				assertFields(t, err, tt.wantFields)
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && tt.wantFields == nil {
				if len(members.transfers) != 1 || members.transfers[0] != [2]uuid.UUID{f.owner, tt.target(f)} {
					t.Errorf("transfers = %v", members.transfers)
				}
			}
		})
	}
}

func TestMembershipService_NotificationSettings(t *testing.T) {
	t.Parallel()

	t.Run("閲覧者も、自分の通知設定を変えられる（要件4-1）", func(t *testing.T) {
		t.Parallel()
		f, members, svc := newMembershipService()
		got, err := svc.UpdateNotificationSettings(t.Context(), f.viewer, f.household, false, "21:30", true)
		if err != nil {
			t.Fatal(err)
		}
		want := domain.NotificationSettings{ReminderEnabled: false, ReminderTime: civil.Time{Hour: 21, Minute: 30}, CompletionNotifyEnabled: true}
		if got != want || len(members.settings) != 1 {
			t.Errorf("got %+v, saved %+v", got, members.settings)
		}
	})

	t.Run("時刻の形式が違えば保存しない", func(t *testing.T) {
		t.Parallel()
		f, members, svc := newMembershipService()
		_, err := svc.UpdateNotificationSettings(t.Context(), f.viewer, f.household, true, "9時", true)
		assertFields(t, err, map[string]string{"reminderTime": "invalid"})
		if len(members.settings) != 0 {
			t.Error("repository was called")
		}
	})

	t.Run("メンバーでなければ、見つからない（不正な入力でも）", func(t *testing.T) {
		t.Parallel()
		f, _, svc := newMembershipService()
		if _, err := svc.UpdateNotificationSettings(t.Context(), f.stranger, f.household, true, "9時", true); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("update: err = %v, want ErrNotFound", err)
		}
		if _, err := svc.GetNotificationSettings(t.Context(), f.stranger, f.household); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("get: err = %v, want ErrNotFound", err)
		}
	})
}

// assertFields は、ValidationError の fields が wantFields と一致することを確かめる。
func assertFields(t *testing.T, err error, wantFields map[string]string) {
	t.Helper()
	var verr *domain.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if !maps.Equal(verr.Fields, wantFields) {
		t.Errorf("fields = %v, want %v", verr.Fields, wantFields)
	}
}
