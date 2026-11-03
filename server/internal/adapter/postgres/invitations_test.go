package postgres_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func newSecret(t *testing.T) domain.InvitationSecret {
	t.Helper()
	s, err := domain.NewInvitationSecret()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestInvitationRepository_Create(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newMemberFixture(t)
	repo := postgres.NewInvitationRepository(f.pool)
	secret := newSecret(t)

	inv, err := repo.Create(ctx, f.household, f.alice, domain.RoleEditor, secret)
	if err != nil {
		t.Fatal(err)
	}
	if inv.HouseholdID != f.household || inv.Role != domain.RoleEditor || inv.Code != secret.Code {
		t.Errorf("invitation = %+v", inv)
	}
	// 有効期限は作成から7日（要件3-1）
	if d := inv.ExpiresAt.Sub(inv.CreatedAt); d != 7*24*time.Hour {
		t.Errorf("expires in %v, want 7 days", d)
	}

	// DB にはトークンそのものは入らず、ハッシュだけが入る
	var hash []byte
	var tokenFound bool
	// 行のすべての列を JSON にして、トークンの文字列が含まれないことを確かめる
	if err := f.pool.QueryRow(ctx, `SELECT token_hash, position($2 in row_to_json(invitations)::text) > 0 FROM invitations WHERE id = $1`, inv.ID, secret.Token).Scan(&hash, &tokenFound); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(hash, secret.TokenHash) || tokenFound {
		t.Errorf("stored token_hash does not match, or the raw token was stored")
	}
}

// コードがほかの招待と重なったら、ErrInvitationCodeTaken を返す（サービスが作り直す）。
func TestInvitationRepository_Create_CodeTaken(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newMemberFixture(t)
	repo := postgres.NewInvitationRepository(f.pool)

	first := newSecret(t)
	if _, err := repo.Create(ctx, f.household, f.alice, domain.RoleEditor, first); err != nil {
		t.Fatal(err)
	}
	second := newSecret(t)
	second.Code = first.Code
	if _, err := repo.Create(ctx, f.household, f.alice, domain.RoleViewer, second); !errors.Is(err, app.ErrInvitationCodeTaken) {
		t.Errorf("err = %v, want ErrInvitationCodeTaken", err)
	}
}

func TestInvitationRepository_ListActiveAndRevoke(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newMemberFixture(t)
	repo := postgres.NewInvitationRepository(f.pool)

	create := func(role domain.Role) domain.Invitation {
		t.Helper()
		inv, err := repo.Create(ctx, f.household, f.alice, role, newSecret(t))
		if err != nil {
			t.Fatal(err)
		}
		return inv
	}
	older := create(domain.RoleEditor)
	newer := create(domain.RoleViewer)
	revoked := create(domain.RoleViewer)
	expired := create(domain.RoleViewer)
	if _, err := f.pool.Exec(ctx, `UPDATE invitations SET expires_at = now() - interval '1 second' WHERE id = $1`, expired.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Revoke(ctx, f.household, revoked.ID); err != nil {
		t.Fatal(err)
	}
	// ほかの世帯の招待は含めない
	otherHousehold, err := postgres.NewHouseholdRepository(f.pool).CreateWithOwner(ctx, f.dave, domain.NewHousehold{Name: "デイブの家"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, otherHousehold.ID, f.dave, domain.RoleEditor, newSecret(t)); err != nil {
		t.Fatal(err)
	}

	got, err := repo.ListActive(ctx, f.household)
	if err != nil {
		t.Fatal(err)
	}
	// 取り消し済み・期限切れを除き、新しい順に返す
	if len(got) != 2 || got[0].ID != newer.ID || got[1].ID != older.ID {
		t.Errorf("active = %+v, want [newer, older]", got)
	}

	// 取り消しは何度でもでき、最初に取り消した日時を保つ
	var firstRevokedAt time.Time
	if err := f.pool.QueryRow(ctx, `SELECT revoked_at FROM invitations WHERE id = $1`, revoked.ID).Scan(&firstRevokedAt); err != nil {
		t.Fatal(err)
	}
	if err := repo.Revoke(ctx, f.household, revoked.ID); err != nil {
		t.Errorf("second revoke: %v", err)
	}
	var revokedAt time.Time
	if err := f.pool.QueryRow(ctx, `SELECT revoked_at FROM invitations WHERE id = $1`, revoked.ID).Scan(&revokedAt); err != nil {
		t.Fatal(err)
	}
	if !revokedAt.Equal(firstRevokedAt) {
		t.Errorf("revoked_at changed from %v to %v", firstRevokedAt, revokedAt)
	}

	// ほかの世帯の招待・存在しない招待は、見つからない
	if err := repo.Revoke(ctx, otherHousehold.ID, older.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other household: err = %v, want ErrNotFound", err)
	}
	if err := repo.Revoke(ctx, f.household, uuid.Must(uuid.NewV7())); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown invitation: err = %v, want ErrNotFound", err)
	}
	if got, _ := repo.ListActive(ctx, f.household); len(got) != 2 {
		t.Errorf("revoking from another household changed this household's invitations: %+v", got)
	}
}
