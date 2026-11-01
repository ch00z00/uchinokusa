package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

var testMemberID = uuid.MustParse("01920000-0000-7000-8000-0000000000b1")

// fakeMembershipService は、app.MembershipService の偽物である。
// testUser は testHousehold のオーナーで、testMemberID は編集者とする。
type fakeMembershipService struct {
	removed   []uuid.UUID
	transfers []uuid.UUID
}

func (f *fakeMembershipService) List(_ context.Context, _, _ uuid.UUID) ([]domain.Member, error) {
	joined := time.Date(2026, 11, 2, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	return []domain.Member{
		{UserID: testUser.ID, DisplayName: "ゆうすけ", Role: domain.RoleOwner, JoinedAt: joined},
		{UserID: testMemberID, DisplayName: "はなこ", Role: domain.RoleEditor, JoinedAt: joined},
	}, nil
}

func (f *fakeMembershipService) UpdateRole(_ context.Context, actorID, _, targetID uuid.UUID, role string) (domain.Member, error) {
	if targetID == actorID {
		return domain.Member{}, &domain.ConflictError{Code: domain.ConflictLastOwner}
	}
	r, err := domain.ParseAssignableRole(role)
	if err != nil {
		return domain.Member{}, err
	}
	return domain.Member{UserID: targetID, DisplayName: "はなこ", Role: r}, nil
}

func (f *fakeMembershipService) Remove(_ context.Context, actorID, _, targetID uuid.UUID) error {
	if targetID == actorID {
		return &domain.ConflictError{Code: domain.ConflictLastOwner}
	}
	f.removed = append(f.removed, targetID)
	return nil
}

func (f *fakeMembershipService) TransferOwnership(_ context.Context, _, _, targetID uuid.UUID) error {
	if targetID != testMemberID {
		return &domain.ValidationError{Fields: map[string]string{"userId": "not_member"}}
	}
	f.transfers = append(f.transfers, targetID)
	return nil
}

func (f *fakeMembershipService) GetNotificationSettings(_ context.Context, _, _ uuid.UUID) (domain.NotificationSettings, error) {
	return domain.NotificationSettings{ReminderEnabled: true, ReminderTime: civil.Time{Hour: 8}, CompletionNotifyEnabled: true}, nil
}

func (f *fakeMembershipService) UpdateNotificationSettings(_ context.Context, _, _ uuid.UUID, reminderEnabled bool, reminderTime string, completionNotifyEnabled bool) (domain.NotificationSettings, error) {
	return domain.NewNotificationSettings(reminderEnabled, reminderTime, completionNotifyEnabled)
}

func newMemberRouter(t *testing.T, members *fakeMembershipService) http.Handler {
	t.Helper()
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Households: &fakeHouseholdService{}, Members: members})
}

func membersPath(rest string) string {
	return "/v1/households/" + testHousehold.ID.String() + rest
}

type memberBody struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	JoinedAt    string `json:"joinedAt"`
}

func TestListMembers(t *testing.T) {
	t.Parallel()
	rec := serve(t, newMemberRouter(t, &fakeMembershipService{}), http.MethodGet, membersPath("/members"), "Bearer "+validToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	got := decode[struct{ Members []memberBody }](t, rec).Members
	if len(got) != 2 || got[1].DisplayName != "はなこ" || got[1].Role != "editor" || got[1].JoinedAt != "2026-11-02T00:00:00Z" {
		t.Errorf("members = %+v", got)
	}
}

func TestUpdateMemberRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		target     uuid.UUID
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "ロールを変える", target: testMemberID, body: `{"role": "viewer"}`, wantStatus: 200},
		{name: "owner は指定できない", target: testMemberID, body: `{"role": "owner"}`, wantStatus: 422, wantCode: "validation_failed"},
		{name: "オーナー自身は 409 last_owner", target: testUser.ID, body: `{"role": "editor"}`, wantStatus: 409, wantCode: "last_owner"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := serve(t, newMemberRouter(t, &fakeMembershipService{}), http.MethodPatch, membersPath("/members/"+tt.target.String()), "Bearer "+validToken, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantCode != "" {
				if got := decode[errorBody](t, rec).Code; got != tt.wantCode {
					t.Errorf("code = %s, want %s", got, tt.wantCode)
				}
				return
			}
			if got := decode[memberBody](t, rec); got.Role != "viewer" || got.UserID != testMemberID.String() {
				t.Errorf("member = %+v", got)
			}
		})
	}
}

func TestRemoveMember(t *testing.T) {
	t.Parallel()

	t.Run("外したら 204", func(t *testing.T) {
		t.Parallel()
		members := &fakeMembershipService{}
		rec := serve(t, newMemberRouter(t, members), http.MethodDelete, membersPath("/members/"+testMemberID.String()), "Bearer "+validToken, "")
		if rec.Code != http.StatusNoContent || len(members.removed) != 1 {
			t.Errorf("status = %d, removed = %v", rec.Code, members.removed)
		}
	})

	t.Run("オーナーの退出は 409 last_owner", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newMemberRouter(t, &fakeMembershipService{}), http.MethodDelete, membersPath("/members/"+testUser.ID.String()), "Bearer "+validToken, "")
		if rec.Code != http.StatusConflict || decode[errorBody](t, rec).Code != "last_owner" {
			t.Errorf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})
}

func TestTransferOwnership(t *testing.T) {
	t.Parallel()

	t.Run("移したら 204", func(t *testing.T) {
		t.Parallel()
		members := &fakeMembershipService{}
		rec := serve(t, newMemberRouter(t, members), http.MethodPost, membersPath("/ownership-transfer"), "Bearer "+validToken, `{"userId": "`+testMemberID.String()+`"}`)
		if rec.Code != http.StatusNoContent || len(members.transfers) != 1 {
			t.Errorf("status = %d (%s), transfers = %v", rec.Code, rec.Body.String(), members.transfers)
		}
	})

	t.Run("メンバーでない人には 422 not_member", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newMemberRouter(t, &fakeMembershipService{}), http.MethodPost, membersPath("/ownership-transfer"), "Bearer "+validToken, `{"userId": "`+uuid.Must(uuid.NewV7()).String()+`"}`)
		if rec.Code != http.StatusUnprocessableEntity || decode[errorBody](t, rec).Fields["userId"] != "not_member" {
			t.Errorf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})
}

type settingsBody struct {
	ReminderEnabled         bool   `json:"reminderEnabled"`
	ReminderTime            string `json:"reminderTime"`
	CompletionNotifyEnabled bool   `json:"completionNotifyEnabled"`
}

func TestNotificationSettings(t *testing.T) {
	t.Parallel()

	t.Run("取得する", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newMemberRouter(t, &fakeMembershipService{}), http.MethodGet, membersPath("/members/me/notification-settings"), "Bearer "+validToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if got := decode[settingsBody](t, rec); got != (settingsBody{ReminderEnabled: true, ReminderTime: "08:00", CompletionNotifyEnabled: true}) {
			t.Errorf("settings = %+v", got)
		}
	})

	t.Run("変更する", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newMemberRouter(t, &fakeMembershipService{}), http.MethodPut, membersPath("/members/me/notification-settings"), "Bearer "+validToken,
			`{"reminderEnabled": false, "reminderTime": "21:30", "completionNotifyEnabled": true}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if got := decode[settingsBody](t, rec); got != (settingsBody{ReminderEnabled: false, ReminderTime: "21:30", CompletionNotifyEnabled: true}) {
			t.Errorf("settings = %+v", got)
		}
	})

	t.Run("時刻の形式が違えば 422", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newMemberRouter(t, &fakeMembershipService{}), http.MethodPut, membersPath("/members/me/notification-settings"), "Bearer "+validToken,
			`{"reminderEnabled": true, "reminderTime": "9:00", "completionNotifyEnabled": true}`)
		if rec.Code != http.StatusUnprocessableEntity || decode[errorBody](t, rec).Fields["reminderTime"] != "invalid" {
			t.Errorf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})
}
