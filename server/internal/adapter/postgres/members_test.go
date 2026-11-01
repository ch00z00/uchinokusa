package postgres_test

import (
	"errors"
	"sync"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/pgtest"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// memberFixture は、オーナー（alice）・編集者（bob）・閲覧者（carol）のいる世帯を用意する。
type memberFixture struct {
	pool                    *pgxpool.Pool
	repo                    *postgres.MemberRepository
	household               uuid.UUID
	alice, bob, carol, dave uuid.UUID // dave はメンバーではない
}

func newMemberFixture(t *testing.T) memberFixture {
	t.Helper()
	pool := pgtest.New(t)
	f := memberFixture{pool: pool, repo: postgres.NewMemberRepository(pool)}
	f.alice = createUserNamed(t, pool, "uid-alice", "アリス")
	f.bob = createUserNamed(t, pool, "uid-bob", "ボブ")
	f.carol = createUserNamed(t, pool, "uid-carol", "キャロル")
	f.dave = createUserNamed(t, pool, "uid-dave", "デイブ")
	h, err := postgres.NewHouseholdRepository(pool).CreateWithOwner(t.Context(), f.alice, domain.NewHousehold{Name: "自宅"})
	if err != nil {
		t.Fatal(err)
	}
	f.household = h.ID
	addMember(t, pool, f.household, f.bob, domain.RoleEditor)
	addMember(t, pool, f.household, f.carol, domain.RoleViewer)
	return f
}

func createUserNamed(t *testing.T, pool *pgxpool.Pool, uid, name string) uuid.UUID {
	t.Helper()
	u, err := postgres.NewUserRepository(pool).GetOrCreate(t.Context(), domain.Identity{UID: uid, Name: name, Provider: "google.com"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func (f memberFixture) roleOf(t *testing.T, userID uuid.UUID) domain.Role {
	t.Helper()
	m, err := postgres.NewMembershipRepository(f.pool).Get(t.Context(), f.household, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return m.Role
}

func (f memberFixture) ownerCount(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM memberships WHERE household_id = $1 AND role = 'owner'`, f.household).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func isLastOwner(err error) bool {
	var c *domain.ConflictError
	return errors.As(err, &c) && c.Code == domain.ConflictLastOwner
}

func TestMemberRepository_List(t *testing.T) {
	t.Parallel()
	f := newMemberFixture(t)

	got, err := f.repo.List(t.Context(), f.household)
	if err != nil {
		t.Fatal(err)
	}
	// 参加の古い順に、表示名とロールを返す
	want := []struct {
		name string
		role domain.Role
	}{{"アリス", domain.RoleOwner}, {"ボブ", domain.RoleEditor}, {"キャロル", domain.RoleViewer}}
	if len(got) != len(want) {
		t.Fatalf("members = %+v", got)
	}
	for i, w := range want {
		if got[i].DisplayName != w.name || got[i].Role != w.role || got[i].JoinedAt.IsZero() {
			t.Errorf("members[%d] = %+v, want %s %s", i, got[i], w.name, w.role)
		}
	}
}

func TestMemberRepository_UpdateRole(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newMemberFixture(t)

	m, err := f.repo.UpdateRole(ctx, f.household, f.bob, domain.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	if m.UserID != f.bob || m.DisplayName != "ボブ" || m.Role != domain.RoleViewer {
		t.Errorf("member = %+v", m)
	}

	if _, err := f.repo.UpdateRole(ctx, f.household, f.alice, domain.RoleEditor); !isLastOwner(err) {
		t.Errorf("owner: err = %v, want last_owner", err)
	}
	if f.roleOf(t, f.alice) != domain.RoleOwner {
		t.Error("owner's role was changed")
	}
	if _, err := f.repo.UpdateRole(ctx, f.household, f.dave, domain.RoleEditor); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("non-member: err = %v, want ErrNotFound", err)
	}
}

// 外したメンバーの過去の記録は、作業者名を残したまま残る（要件4-7・12-5）。
func TestMemberRepository_Remove_KeepsRecords(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newMemberFixture(t)

	var recordID uuid.UUID
	err := f.pool.QueryRow(ctx, `
		WITH p AS (INSERT INTO plants (household_id, name) VALUES ($1, 'ポトス') RETURNING id)
		INSERT INTO care_records (household_id, plant_id, task_kind, outcome, performed_on, actor_user_id, actor_display_name, cycle_seq)
		SELECT $1, p.id, 'watering', 'completed', '2026-11-01', $2, 'ボブ', 1 FROM p
		RETURNING id`, f.household, f.bob).Scan(&recordID)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.repo.Remove(ctx, f.household, f.bob); err != nil {
		t.Fatal(err)
	}
	if f.roleOf(t, f.bob) != "" {
		t.Error("bob is still a member")
	}
	var actorID uuid.NullUUID
	var actorName string
	if err := f.pool.QueryRow(ctx, `SELECT actor_user_id, actor_display_name FROM care_records WHERE id = $1`, recordID).Scan(&actorID, &actorName); err != nil {
		t.Fatalf("record was removed with the member: %v", err)
	}
	if actorName != "ボブ" || actorID.UUID != f.bob {
		t.Errorf("record actor = %v %q, want bob", actorID, actorName)
	}

	if err := f.repo.Remove(ctx, f.household, f.alice); !isLastOwner(err) {
		t.Errorf("owner: err = %v, want last_owner", err)
	}
	if err := f.repo.Remove(ctx, f.household, f.dave); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("non-member: err = %v, want ErrNotFound", err)
	}
}

func TestMemberRepository_TransferOwnership(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newMemberFixture(t)

	if err := f.repo.TransferOwnership(ctx, f.household, f.alice, f.carol); err != nil {
		t.Fatal(err)
	}
	// 移した先がオーナーになり、元のオーナーは編集者になる（要件4-8）
	if f.roleOf(t, f.carol) != domain.RoleOwner || f.roleOf(t, f.alice) != domain.RoleEditor {
		t.Errorf("roles: carol=%s alice=%s, want owner/editor", f.roleOf(t, f.carol), f.roleOf(t, f.alice))
	}

	// もうオーナーではない alice は移せない
	if err := f.repo.TransferOwnership(ctx, f.household, f.alice, f.bob); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("former owner: err = %v, want ErrForbidden", err)
	}
	// メンバーでない人には移せず、何も変わらない
	if err := f.repo.TransferOwnership(ctx, f.household, f.carol, f.dave); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("non-member: err = %v, want ErrNotFound", err)
	}
	if f.roleOf(t, f.carol) != domain.RoleOwner || f.ownerCount(t) != 1 {
		t.Error("roles changed after a failed transfer")
	}
}

// 同じオーナーが、複数の人への移譲を同時に送っても、オーナーは1人だけになる。
func TestMemberRepository_TransferOwnership_Concurrent(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newMemberFixture(t)
	targets := []uuid.UUID{f.bob, f.carol}
	for i := range 6 {
		u := createUserNamed(t, f.pool, "uid-extra-"+string(rune('a'+i)), "メンバー")
		addMember(t, f.pool, f.household, u, domain.RoleEditor)
		targets = append(targets, u)
	}

	errs := make([]error, len(targets))
	var wg sync.WaitGroup
	for i, to := range targets {
		wg.Go(func() { errs[i] = f.repo.TransferOwnership(ctx, f.household, f.alice, to) })
	}
	wg.Wait()

	succeeded := 0
	for i, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case !errors.Is(err, domain.ErrForbidden):
			t.Errorf("transfer %d: unexpected error %v", i, err)
		}
	}
	if succeeded != 1 || f.ownerCount(t) != 1 {
		t.Errorf("succeeded = %d, owners = %d, want 1 and 1", succeeded, f.ownerCount(t))
	}
	if f.roleOf(t, f.alice) != domain.RoleEditor {
		t.Errorf("alice = %s, want editor", f.roleOf(t, f.alice))
	}
}

func TestMemberRepository_NotificationSettings(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	f := newMemberFixture(t)

	got, err := f.repo.GetNotificationSettings(ctx, f.household, f.carol)
	if err != nil {
		t.Fatal(err)
	}
	// 初期値：リマインダーは 08:00 に受け取る、完了通知も受け取る（要件16-3）
	if want := (domain.NotificationSettings{ReminderEnabled: true, ReminderTime: civil.Time{Hour: 8}, CompletionNotifyEnabled: true}); got != want {
		t.Errorf("default = %+v, want %+v", got, want)
	}

	want := domain.NotificationSettings{ReminderEnabled: false, ReminderTime: civil.Time{Hour: 21, Minute: 30}, CompletionNotifyEnabled: false}
	updated, err := f.repo.UpdateNotificationSettings(ctx, f.household, f.carol, want)
	if err != nil {
		t.Fatal(err)
	}
	if updated != want {
		t.Errorf("updated = %+v, want %+v", updated, want)
	}
	// 世帯ごと・メンバーごとの設定なので、ほかのメンバーは変わらない
	if other, _ := f.repo.GetNotificationSettings(ctx, f.household, f.bob); !other.ReminderEnabled {
		t.Error("bob's settings were changed")
	}

	if _, err := f.repo.GetNotificationSettings(ctx, f.household, f.dave); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("non-member: err = %v, want ErrNotFound", err)
	}
}
