package postgres_test

import (
	"bytes"
	"errors"
	"sync"
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

// joinFixture は、オーナー alice の世帯に、編集者として参加できる有効な招待を1つ用意する。
type joinFixture struct {
	memberFixture
	repo   *postgres.InvitationRepository
	secret domain.InvitationSecret
	inv    domain.Invitation
}

func newJoinFixture(t *testing.T) joinFixture {
	t.Helper()
	f := joinFixture{memberFixture: newMemberFixture(t)}
	f.repo = postgres.NewInvitationRepository(f.pool)
	f.secret = newSecret(t)
	inv, err := f.repo.Create(t.Context(), f.household, f.alice, domain.RoleEditor, f.secret)
	if err != nil {
		t.Fatal(err)
	}
	f.inv = inv
	return f
}

func (f joinFixture) outboxCount(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM notification_outbox WHERE household_id = $1 AND kind = 'member_joined' AND actor_user_id = $2`, f.household, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestInvitationRepository_Preview(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newJoinFixture(t)

	for name, key := range map[string]domain.InvitationKey{
		"コード":  {Code: f.secret.Code},
		"トークン": {TokenHash: f.secret.TokenHash},
	} {
		p, err := f.repo.Preview(ctx, key, f.dave)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if p.HouseholdName != "自宅" || p.Role != domain.RoleEditor || p.AlreadyMember || !p.ExpiresAt.Equal(f.inv.ExpiresAt) {
			t.Errorf("%s: preview = %+v", name, p)
		}
	}

	// すでにメンバーなら、そのことを返す（要件3-6）
	if p, err := f.repo.Preview(ctx, domain.InvitationKey{Code: f.secret.Code}, f.bob); err != nil || !p.AlreadyMember {
		t.Errorf("member: preview = %+v, err = %v", p, err)
	}
}

// 期限切れ・取り消し済み・存在しない招待は、区別せずに無効にする（要件3-5）。
func TestInvitationRepository_InvalidInvitations(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newJoinFixture(t)

	expired := newSecret(t)
	expiredInv, err := f.repo.Create(ctx, f.household, f.alice, domain.RoleViewer, expired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE invitations SET expires_at = now() - interval '1 second' WHERE id = $1`, expiredInv.ID); err != nil {
		t.Fatal(err)
	}
	revoked := newSecret(t)
	revokedInv, err := f.repo.Create(ctx, f.household, f.alice, domain.RoleViewer, revoked)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.repo.Revoke(ctx, f.household, revokedInv.ID); err != nil {
		t.Fatal(err)
	}

	for name, key := range map[string]domain.InvitationKey{
		"期限切れ":      {Code: expired.Code},
		"取り消し済み":    {TokenHash: revoked.TokenHash},
		"存在しないコード":  {Code: "AAAAAAAA"},
		"存在しないトークン": {TokenHash: domain.HashInvitationToken("unknown")},
		"鍵がない（両方空）": {},
	} {
		if _, err := f.repo.Preview(ctx, key, f.dave); !errors.Is(err, domain.ErrInvitationInvalid) {
			t.Errorf("%s: preview err = %v, want ErrInvitationInvalid", name, err)
		}
		if _, err := f.repo.Accept(ctx, key, f.dave); !errors.Is(err, domain.ErrInvitationInvalid) {
			t.Errorf("%s: accept err = %v, want ErrInvitationInvalid", name, err)
		}
	}
	if f.roleOf(t, f.dave) != "" || f.outboxCount(t, f.dave) != 0 {
		t.Error("dave joined through an invalid invitation")
	}
}

func TestInvitationRepository_Accept(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newJoinFixture(t)

	h, err := f.repo.Accept(ctx, domain.InvitationKey{TokenHash: f.secret.TokenHash}, f.dave)
	if err != nil {
		t.Fatal(err)
	}
	// 招待のロールで世帯に追加する（要件3-3）
	if h.ID != f.household || h.Name != "自宅" || h.Role != domain.RoleEditor || f.roleOf(t, f.dave) != domain.RoleEditor {
		t.Errorf("household = %+v, role = %s", h, f.roleOf(t, f.dave))
	}
	// オーナーへの参加の通知を予約する（要件3-8）
	if n := f.outboxCount(t, f.dave); n != 1 {
		t.Errorf("outbox = %d, want 1", n)
	}

	// 有効期限内は、ほかの人も同じ招待を使える
	erin := createUserNamed(t, f.pool, "uid-erin", "エリン")
	if _, err := f.repo.Accept(ctx, domain.InvitationKey{Code: f.secret.Code}, erin); err != nil {
		t.Errorf("second user: %v", err)
	}

	// すでにメンバーなら、重複して追加せず、通知も積まない（要件3-6）
	_, err = f.repo.Accept(ctx, domain.InvitationKey{Code: f.secret.Code}, f.dave)
	var c *domain.ConflictError
	if !errors.As(err, &c) || c.Code != domain.ConflictAlreadyMember {
		t.Errorf("again: err = %v, want already_member", err)
	}
	if n := f.outboxCount(t, f.dave); n != 1 {
		t.Errorf("outbox after the second accept = %d, want 1", n)
	}
	// オーナー（alice）が自分の世帯の招待を使っても、ロールは変わらない
	if _, err := f.repo.Accept(ctx, domain.InvitationKey{Code: f.secret.Code}, f.alice); !errors.As(err, &c) {
		t.Errorf("owner: err = %v, want already_member", err)
	}
	if f.roleOf(t, f.alice) != domain.RoleOwner {
		t.Errorf("owner's role changed to %s", f.roleOf(t, f.alice))
	}
}

// 同じ人の参加が同時に届いても、メンバーシップと通知は1つだけになる。
func TestInvitationRepository_Accept_Concurrent(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newJoinFixture(t)

	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { _, errs[i] = f.repo.Accept(ctx, domain.InvitationKey{Code: f.secret.Code}, f.dave) })
	}
	wg.Wait()

	succeeded := 0
	for i, err := range errs {
		var c *domain.ConflictError
		switch {
		case err == nil:
			succeeded++
		case !errors.As(err, &c) || c.Code != domain.ConflictAlreadyMember:
			t.Errorf("accept %d: unexpected error %v", i, err)
		}
	}
	if succeeded != 1 || f.outboxCount(t, f.dave) != 1 {
		t.Errorf("succeeded = %d, outbox = %d, want 1 and 1", succeeded, f.outboxCount(t, f.dave))
	}
}
