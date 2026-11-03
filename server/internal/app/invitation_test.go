package app_test

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeInvitations は、app.InvitationRepository の偽物である。
// codeTaken の回数だけ、コードの重複で作成に失敗する。
type fakeInvitations struct {
	codeTaken int
	attempts  int
	created   []domain.InvitationSecret
	revoked   []uuid.UUID
	keys      []domain.InvitationKey
	joined    []uuid.UUID
}

func (f *fakeInvitations) Create(_ context.Context, householdID, _ uuid.UUID, role domain.Role, s domain.InvitationSecret) (domain.Invitation, error) {
	f.attempts++
	if f.attempts <= f.codeTaken {
		return domain.Invitation{}, app.ErrInvitationCodeTaken
	}
	f.created = append(f.created, s)
	return domain.Invitation{ID: uuid.Must(uuid.NewV7()), HouseholdID: householdID, Role: role, Code: s.Code}, nil
}

func (f *fakeInvitations) ListActive(_ context.Context, _ uuid.UUID) ([]domain.Invitation, error) {
	return []domain.Invitation{}, nil
}

// validCode は、fakeInvitations が有効として扱う唯一の招待コードである。
const validCode = "K7MP2QXA"

func (f *fakeInvitations) Preview(_ context.Context, key domain.InvitationKey, _ uuid.UUID) (domain.InvitationPreview, error) {
	f.keys = append(f.keys, key)
	if key.Code != validCode {
		return domain.InvitationPreview{}, domain.ErrInvitationInvalid
	}
	return domain.InvitationPreview{HouseholdName: "自宅", Role: domain.RoleEditor}, nil
}

func (f *fakeInvitations) Accept(_ context.Context, key domain.InvitationKey, userID uuid.UUID) (domain.HouseholdWithRole, error) {
	f.keys = append(f.keys, key)
	if key.Code != validCode {
		return domain.HouseholdWithRole{}, domain.ErrInvitationInvalid
	}
	f.joined = append(f.joined, userID)
	return domain.HouseholdWithRole{Household: domain.Household{Name: "自宅"}, Role: domain.RoleEditor}, nil
}

func (f *fakeInvitations) Revoke(_ context.Context, _, invitationID uuid.UUID) error {
	f.revoked = append(f.revoked, invitationID)
	return nil
}

func newInvitationService(t *testing.T, invitations *fakeInvitations) (householdFixture, *app.InvitationService) {
	t.Helper()
	f := newHouseholdFixture()
	base, err := url.Parse("https://invite.example.com")
	if err != nil {
		t.Fatal(err)
	}
	return f, app.NewInvitationService(invitations, f.authz, base)
}

func TestInvitationService_Create(t *testing.T) {
	t.Parallel()

	t.Run("招待リンクの URL とコードを返し、トークンのハッシュを保存する", func(t *testing.T) {
		t.Parallel()
		invitations := &fakeInvitations{}
		f, svc := newInvitationService(t, invitations)

		got, err := svc.Create(t.Context(), f.owner, f.household, "editor")
		if err != nil {
			t.Fatal(err)
		}
		if got.Role != domain.RoleEditor || got.HouseholdID != f.household {
			t.Errorf("invitation = %+v", got.Invitation)
		}
		saved := invitations.created[0]
		if got.URL != "https://invite.example.com/i/"+saved.Token {
			t.Errorf("URL = %s, want the link with the token", got.URL)
		}
		if got.Code != saved.Code {
			t.Errorf("code = %s, want %s", got.Code, saved.Code)
		}
	})

	t.Run("コードが重なったら、作り直す", func(t *testing.T) {
		t.Parallel()
		invitations := &fakeInvitations{codeTaken: 2}
		f, svc := newInvitationService(t, invitations)

		if _, err := svc.Create(t.Context(), f.owner, f.household, "viewer"); err != nil {
			t.Fatal(err)
		}
		if invitations.attempts != 3 {
			t.Errorf("attempts = %d, want 3", invitations.attempts)
		}
	})

	t.Run("重なり続けたら、あきらめてエラーにする", func(t *testing.T) {
		t.Parallel()
		invitations := &fakeInvitations{codeTaken: 100}
		f, svc := newInvitationService(t, invitations)

		_, err := svc.Create(t.Context(), f.owner, f.household, "viewer")
		if !errors.Is(err, app.ErrInvitationCodeTaken) {
			t.Errorf("err = %v, want ErrInvitationCodeTaken", err)
		}
		if invitations.attempts > 5 {
			t.Errorf("attempts = %d, want a small bounded number", invitations.attempts)
		}
	})

	tests := []struct {
		name       string
		actor      pick
		role       string
		wantErr    error
		wantFields map[string]string
	}{
		{name: "owner のロールでは招待できない", actor: owner, role: "owner", wantFields: map[string]string{"role": "invalid"}},
		{name: "編集者は招待できない", actor: editor, role: "viewer", wantErr: domain.ErrForbidden},
		{name: "閲覧者は招待できない", actor: viewer, role: "viewer", wantErr: domain.ErrForbidden},
		{name: "メンバーでなければ、見つからない（不正な入力でも）", actor: stranger, role: "owner", wantErr: domain.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			invitations := &fakeInvitations{}
			f, svc := newInvitationService(t, invitations)

			_, err := svc.Create(t.Context(), tt.actor(f), f.household, tt.role)
			if tt.wantFields != nil {
				assertFields(t, err, tt.wantFields)
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if invitations.attempts != 0 {
				t.Error("repository was called")
			}
		})
	}
}

func TestInvitationService_ListAndRevoke(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		actor   pick
		wantErr error
	}{
		{name: "オーナーはできる", actor: owner},
		{name: "編集者はできない", actor: editor, wantErr: domain.ErrForbidden},
		{name: "メンバーでなければ、見つからない", actor: stranger, wantErr: domain.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			invitations := &fakeInvitations{}
			f, svc := newInvitationService(t, invitations)

			if _, err := svc.List(t.Context(), tt.actor(f), f.household); !errors.Is(err, tt.wantErr) {
				t.Errorf("List: err = %v, want %v", err, tt.wantErr)
			}
			if err := svc.Revoke(t.Context(), tt.actor(f), f.household, uuid.Must(uuid.NewV7())); !errors.Is(err, tt.wantErr) {
				t.Errorf("Revoke: err = %v, want %v", err, tt.wantErr)
			}
			if wantRevoked := tt.wantErr == nil; wantRevoked != (len(invitations.revoked) == 1) {
				t.Errorf("revoked = %v", invitations.revoked)
			}
		})
	}
}

func TestInvitationService_PreviewAndAccept(t *testing.T) {
	t.Parallel()

	t.Run("手入力のコードを正規化して探す", func(t *testing.T) {
		t.Parallel()
		invitations := &fakeInvitations{}
		f, svc := newInvitationService(t, invitations)

		p, err := svc.Preview(t.Context(), f.stranger, "k7mp-2qxa")
		if err != nil {
			t.Fatal(err)
		}
		if p.HouseholdName != "自宅" || invitations.keys[0].Code != validCode {
			t.Errorf("preview = %+v, key = %+v", p, invitations.keys[0])
		}
	})

	t.Run("参加すると、招待のロールの世帯を返す", func(t *testing.T) {
		t.Parallel()
		invitations := &fakeInvitations{}
		f, svc := newInvitationService(t, invitations)

		h, err := svc.Accept(t.Context(), f.stranger, validCode)
		if err != nil {
			t.Fatal(err)
		}
		if h.Role != domain.RoleEditor || len(invitations.joined) != 1 || invitations.joined[0] != f.stranger {
			t.Errorf("household = %+v, joined = %v", h, invitations.joined)
		}
	})

	t.Run("無効な招待は ErrInvitationInvalid", func(t *testing.T) {
		t.Parallel()
		f, svc := newInvitationService(t, &fakeInvitations{})
		if _, err := svc.Preview(t.Context(), f.stranger, "AAAAAAAA"); !errors.Is(err, domain.ErrInvitationInvalid) {
			t.Errorf("preview: err = %v", err)
		}
		if _, err := svc.Accept(t.Context(), f.stranger, "some-token"); !errors.Is(err, domain.ErrInvitationInvalid) {
			t.Errorf("accept: err = %v", err)
		}
	})
}
