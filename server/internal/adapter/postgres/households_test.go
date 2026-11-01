package postgres_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/pgtest"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func strPtr(s string) *string { return &s }

func TestHouseholdRepository_CreateWithOwner(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewHouseholdRepository(pool)
	alice := createUser(t, pool, "uid-alice")

	h, err := repo.CreateWithOwner(ctx, alice, domain.NewHousehold{Name: "自宅", Region: domain.Region{Prefecture: "東京都"}})
	if err != nil {
		t.Fatal(err)
	}
	if h.ID.Version() != 7 || h.Name != "自宅" || h.Region != (domain.Region{Prefecture: "東京都"}) {
		t.Errorf("household = %+v", h)
	}
	// 初期値：日本（北半球）
	if h.TimeZone != "Asia/Tokyo" || h.Hemisphere != domain.HemisphereNorth {
		t.Errorf("time zone / hemisphere = %s / %s", h.TimeZone, h.Hemisphere)
	}
	// 未設定の地域は NULL で保存する（空文字にしない）
	var city *string
	if err := pool.QueryRow(ctx, `SELECT region_city FROM households WHERE id = $1`, h.ID).Scan(&city); err != nil {
		t.Fatal(err)
	}
	if city != nil {
		t.Errorf("region_city = %q, want NULL", *city)
	}

	m, err := postgres.NewMembershipRepository(pool).Get(ctx, h.ID, alice)
	if err != nil {
		t.Fatalf("owner membership: %v", err)
	}
	if m.Role != domain.RoleOwner {
		t.Errorf("role = %s, want owner", m.Role)
	}
}

func TestHouseholdRepository_ListByUser(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewHouseholdRepository(pool)
	alice := createUser(t, pool, "uid-alice")
	bob := createUser(t, pool, "uid-bob")

	home, err := repo.CreateWithOwner(ctx, alice, domain.NewHousehold{Name: "自宅"})
	if err != nil {
		t.Fatal(err)
	}
	parents, err := repo.CreateWithOwner(ctx, bob, domain.NewHousehold{Name: "実家"})
	if err != nil {
		t.Fatal(err)
	}
	addMember(t, pool, parents.ID, alice, domain.RoleViewer)
	if _, err := repo.CreateWithOwner(ctx, bob, domain.NewHousehold{Name: "bob の部屋"}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.ListByUser(ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	// 所属する世帯だけを、作成の古い順に、それぞれのロールとともに返す
	if len(got) != 2 {
		t.Fatalf("households = %+v, want 2", got)
	}
	if got[0].ID != home.ID || got[0].Role != domain.RoleOwner || got[1].ID != parents.ID || got[1].Role != domain.RoleViewer {
		t.Errorf("got [%s %s, %s %s], want [自宅 owner, 実家 viewer]", got[0].Name, got[0].Role, got[1].Name, got[1].Role)
	}

	none, err := repo.ListByUser(ctx, createUser(t, pool, "uid-carol"))
	if err != nil {
		t.Fatal(err)
	}
	if none == nil || len(none) != 0 {
		t.Errorf("no households: got %#v, want an empty (non-nil) slice", none)
	}
}

func TestHouseholdRepository_Update(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewHouseholdRepository(pool)
	alice := createUser(t, pool, "uid-alice")
	h, err := repo.CreateWithOwner(ctx, alice, domain.NewHousehold{Name: "自宅", Region: domain.Region{Prefecture: "東京都", City: "世田谷区"}})
	if err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		name  string
		patch domain.HouseholdPatch
		want  domain.Household
	}{
		{name: "名前だけ変える（地域はそのまま）", patch: domain.HouseholdPatch{Name: strPtr("わが家")}, want: domain.Household{Name: "わが家", Region: domain.Region{Prefecture: "東京都", City: "世田谷区"}}},
		{name: "市区町村だけ変える", patch: domain.HouseholdPatch{City: strPtr("杉並区")}, want: domain.Household{Name: "わが家", Region: domain.Region{Prefecture: "東京都", City: "杉並区"}}},
		{name: "空文字で未設定に戻す", patch: domain.HouseholdPatch{Prefecture: strPtr(""), City: strPtr("")}, want: domain.Household{Name: "わが家"}},
		{name: "何も変えない", patch: domain.HouseholdPatch{}, want: domain.Household{Name: "わが家"}},
	}
	for _, s := range steps {
		got, err := repo.Update(ctx, h.ID, s.patch)
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if got.ID != h.ID || got.Name != s.want.Name || got.Region != s.want.Region {
			t.Errorf("%s: got %+v, want name=%s region=%+v", s.name, got, s.want.Name, s.want.Region)
		}
	}

	if _, err := repo.Update(ctx, uuid.Must(uuid.NewV7()), domain.HouseholdPatch{Name: strPtr("x")}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown household: err = %v, want ErrNotFound", err)
	}
}

// 世帯を削除すると、配下のデータもすべて消える（要件2-6）。ほかの世帯のデータは残る。
func TestHouseholdRepository_Delete_Cascades(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewHouseholdRepository(pool)
	alice := createUser(t, pool, "uid-alice")

	seed := func(name string) uuid.UUID {
		t.Helper()
		h, err := repo.CreateWithOwner(ctx, alice, domain.NewHousehold{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		// 置き場所・植物・タスク・記録・メモ・招待を1件ずつ入れる
		_, err = pool.Exec(ctx, `
			WITH l AS (INSERT INTO locations (household_id, name) VALUES ($1, '窓際') RETURNING id),
			p AS (INSERT INTO plants (household_id, name, location_id) SELECT $1, 'モンステラ', l.id FROM l RETURNING id),
			tk AS (INSERT INTO care_tasks (household_id, plant_id, kind, interval_value, interval_unit, next_due_on)
			       SELECT $1, p.id, 'watering', 1, 'week', '2026-11-08' FROM p RETURNING id, plant_id),
			r AS (INSERT INTO care_records (household_id, plant_id, task_id, task_kind, outcome, performed_on, actor_user_id, actor_display_name, cycle_seq)
			      SELECT $1, tk.plant_id, tk.id, 'watering', 'completed', '2026-11-01', $2, 'alice', 1 FROM tk),
			n AS (INSERT INTO notes (household_id, plant_id, author_user_id, author_display_name, body)
			      SELECT $1, p.id, $2, 'alice', '新しい葉が出た' FROM p)
			INSERT INTO invitations (household_id, role, token_hash, code, created_by)
			VALUES ($1, 'editor', sha256(convert_to($1::text, 'UTF8')), substr(md5($1::text), 1, 8), $2)`,
			h.ID, alice)
		if err != nil {
			t.Fatal(err)
		}
		return h.ID
	}
	target := seed("自宅")
	other := seed("実家")

	if err := repo.Delete(ctx, target); err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{"households", "memberships", "invitations", "locations", "plants", "care_tasks", "care_records", "notes"} {
		col := "household_id"
		if table == "households" {
			col = "id"
		}
		var deleted, kept int
		q := `SELECT count(*) FILTER (WHERE ` + col + ` = $1), count(*) FILTER (WHERE ` + col + ` = $2) FROM ` + table
		if err := pool.QueryRow(ctx, q, target, other).Scan(&deleted, &kept); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		if deleted != 0 || kept != 1 {
			t.Errorf("%s: deleted household has %d rows (want 0), other household has %d (want 1)", table, deleted, kept)
		}
	}
}
