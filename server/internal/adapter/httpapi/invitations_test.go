package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

var testInvitation = domain.Invitation{
	ID:          uuid.MustParse("01920000-0000-7000-8000-0000000000c1"),
	HouseholdID: testHousehold.ID,
	Role:        domain.RoleEditor,
	Code:        "K7MP2QXA",
	ExpiresAt:   time.Date(2026, 11, 10, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
	CreatedAt:   time.Date(2026, 11, 3, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
}

// fakeInvitationService は、app.InvitationService の偽物である。
type fakeInvitationService struct {
	revoked []uuid.UUID
}

func (f *fakeInvitationService) Create(_ context.Context, _, _ uuid.UUID, role string) (app.CreatedInvitation, error) {
	r, err := domain.ParseAssignableRole(role)
	if err != nil {
		return app.CreatedInvitation{}, err
	}
	inv := testInvitation
	inv.Role = r
	return app.CreatedInvitation{Invitation: inv, URL: "https://invite.example.com/i/token123"}, nil
}

func (f *fakeInvitationService) List(_ context.Context, _, _ uuid.UUID) ([]domain.Invitation, error) {
	return []domain.Invitation{testInvitation}, nil
}

func (f *fakeInvitationService) Preview(_ context.Context, _ uuid.UUID, tokenOrCode string) (domain.InvitationPreview, error) {
	if tokenOrCode != "K7MP2QXA" {
		return domain.InvitationPreview{}, domain.ErrInvitationInvalid
	}
	return domain.InvitationPreview{HouseholdName: "自宅", Role: domain.RoleEditor, ExpiresAt: testInvitation.ExpiresAt}, nil
}

func (f *fakeInvitationService) Accept(_ context.Context, _ uuid.UUID, tokenOrCode string) (domain.HouseholdWithRole, error) {
	switch tokenOrCode {
	case "K7MP2QXA":
		h := testHousehold
		h.Role = domain.RoleEditor
		return h, nil
	case "MEMBER22":
		return domain.HouseholdWithRole{}, &domain.ConflictError{Code: domain.ConflictAlreadyMember}
	}
	return domain.HouseholdWithRole{}, domain.ErrInvitationInvalid
}

func (f *fakeInvitationService) Revoke(_ context.Context, _, _, invitationID uuid.UUID) error {
	f.revoked = append(f.revoked, invitationID)
	return nil
}

func newInvitationRouter(t *testing.T, invitations *fakeInvitationService) http.Handler {
	t.Helper()
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Invitations: invitations})
}

type invitationBody struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Code      string `json:"code"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
	URL       string `json:"url"`
}

func TestCreateInvitation(t *testing.T) {
	t.Parallel()

	t.Run("招待リンクの URL とコードを 201 で返す", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newInvitationRouter(t, &fakeInvitationService{}), http.MethodPost, membersPath("/invitations"), "Bearer "+validToken, `{"role": "viewer"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (%s)", rec.Code, rec.Body.String())
		}
		got := decode[invitationBody](t, rec)
		want := invitationBody{
			ID:        testInvitation.ID.String(),
			Role:      "viewer",
			Code:      "K7MP2QXA",
			ExpiresAt: "2026-11-10T00:00:00Z",
			CreatedAt: "2026-11-03T00:00:00Z",
			URL:       "https://invite.example.com/i/token123",
		}
		if got != want {
			t.Errorf("body = %+v, want %+v", got, want)
		}
	})

	t.Run("owner のロールは 422", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newInvitationRouter(t, &fakeInvitationService{}), http.MethodPost, membersPath("/invitations"), "Bearer "+validToken, `{"role": "owner"}`)
		if rec.Code != http.StatusUnprocessableEntity || decode[errorBody](t, rec).Fields["role"] != "invalid" {
			t.Errorf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})
}

func TestListInvitations(t *testing.T) {
	t.Parallel()
	rec := serve(t, newInvitationRouter(t, &fakeInvitationService{}), http.MethodGet, membersPath("/invitations"), "Bearer "+validToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	got := decode[struct{ Invitations []invitationBody }](t, rec).Invitations
	// 一覧には URL を含めない（トークンを保存していないため）
	if len(got) != 1 || got[0].Code != "K7MP2QXA" || got[0].URL != "" {
		t.Errorf("invitations = %+v", got)
	}
}

func TestRevokeInvitation(t *testing.T) {
	t.Parallel()
	invitations := &fakeInvitationService{}
	rec := serve(t, newInvitationRouter(t, invitations), http.MethodDelete, membersPath("/invitations/"+testInvitation.ID.String()), "Bearer "+validToken, "")
	if rec.Code != http.StatusNoContent || len(invitations.revoked) != 1 || invitations.revoked[0] != testInvitation.ID {
		t.Errorf("status = %d, revoked = %v", rec.Code, invitations.revoked)
	}
}

func TestGetInvitation(t *testing.T) {
	t.Parallel()

	t.Run("世帯名とロールを返す", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newInvitationRouter(t, &fakeInvitationService{}), http.MethodGet, "/v1/invitations/K7MP2QXA", "Bearer "+validToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		got := decode[struct {
			HouseholdName string `json:"householdName"`
			Role          string `json:"role"`
			ExpiresAt     string `json:"expiresAt"`
			AlreadyMember *bool  `json:"alreadyMember"`
		}](t, rec)
		if got.HouseholdName != "自宅" || got.Role != "editor" || got.ExpiresAt != "2026-11-10T00:00:00Z" || got.AlreadyMember == nil || *got.AlreadyMember {
			t.Errorf("body = %+v", got)
		}
	})

	t.Run("無効な招待は 410 invitation_invalid", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newInvitationRouter(t, &fakeInvitationService{}), http.MethodGet, "/v1/invitations/AAAAAAAA", "Bearer "+validToken, "")
		if rec.Code != http.StatusGone || decode[errorBody](t, rec).Code != "invitation_invalid" {
			t.Errorf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})
}

func TestAcceptInvitation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		code       string
		wantStatus int
		wantCode   string
	}{
		{name: "参加した世帯を返す", code: "K7MP2QXA", wantStatus: 200},
		{name: "すでにメンバーなら 409 already_member", code: "MEMBER22", wantStatus: 409, wantCode: "already_member"},
		{name: "無効な招待は 410", code: "AAAAAAAA", wantStatus: 410, wantCode: "invitation_invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := serve(t, newInvitationRouter(t, &fakeInvitationService{}), http.MethodPost, "/v1/invitations/"+tt.code+"/accept", "Bearer "+validToken, "")
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantCode != "" {
				if got := decode[errorBody](t, rec).Code; got != tt.wantCode {
					t.Errorf("code = %s, want %s", got, tt.wantCode)
				}
				return
			}
			if got := decode[householdBody](t, rec); got.ID != testHousehold.ID.String() || got.Role != "editor" {
				t.Errorf("household = %+v", got)
			}
		})
	}
}

// 招待の確認と参加は、あわせて10回まで続けて呼べ、それを超えると 429 になる（要件20-3）。
func TestInvitationRateLimit(t *testing.T) {
	t.Parallel()
	router := newInvitationRouter(t, &fakeInvitationService{})

	for i := range 10 {
		method, path := http.MethodGet, "/v1/invitations/AAAAAAAA"
		if i%2 == 1 {
			method, path = http.MethodPost, "/v1/invitations/AAAAAAAA/accept"
		}
		if rec := serve(t, router, method, path, "Bearer "+validToken, ""); rec.Code != http.StatusGone {
			t.Fatalf("request %d: status = %d, want 410", i+1, rec.Code)
		}
	}
	rec := serve(t, router, http.MethodGet, "/v1/invitations/K7MP2QXA", "Bearer "+validToken, "")
	if rec.Code != http.StatusTooManyRequests || decode[errorBody](t, rec).Code != "rate_limited" {
		t.Fatalf("11th request: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Retry-After"); got != "6" {
		t.Errorf("Retry-After = %q, want 6", got)
	}

	// ほかの操作には制限をかけない
	if rec := serve(t, router, http.MethodGet, membersPath("/invitations"), "Bearer "+validToken, ""); rec.Code != http.StatusOK {
		t.Errorf("list invitations: status = %d, want 200", rec.Code)
	}
}
