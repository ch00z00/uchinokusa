package postgres_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/pgtest"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// createPlant は、置き場所に植物を作る。archived なら、アーカイブした植物にする。
func createPlant(t *testing.T, pool *pgxpool.Pool, householdID, locationID uuid.UUID, archived bool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(t.Context(),
		`INSERT INTO plants (household_id, name, location_id, archived_at) VALUES ($1, 'ポトス', $2, CASE WHEN $3 THEN now() END) RETURNING id`,
		householdID, locationID, archived).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestLocationRepository_CreateAndList(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewLocationRepository(pool)
	home, other := createHousehold(t, pool), createHousehold(t, pool)

	living, err := repo.Create(ctx, home, domain.LocationProfile{
		Name: "リビングの南窓", IsIndoor: true,
		WindowDirection: domain.DirectionSouth, WindowDistance: domain.WindowNear, Shading: domain.ShadingSheer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if living.ID == uuid.Nil || living.HouseholdID != home || living.PlantCount != 0 || living.CreatedAt.IsZero() {
		t.Errorf("created = %+v", living)
	}
	balcony, err := repo.Create(ctx, home, domain.LocationProfile{Name: "ベランダ", IsIndoor: false})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, other, domain.LocationProfile{Name: "ほかの世帯", IsIndoor: true}); err != nil {
		t.Fatal(err)
	}
	createPlant(t, pool, home, living.ID, false)
	createPlant(t, pool, home, living.ID, false)
	createPlant(t, pool, home, living.ID, true) // アーカイブした植物は数えない

	got, err := repo.List(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("locations = %+v, want 2 (only this household)", got)
	}
	// 作成の古い順。未設定の環境の項目は空文字で読み出せる
	if got[0].ID != living.ID || got[0].PlantCount != 2 || got[0].LocationProfile != living.LocationProfile {
		t.Errorf("living = %+v", got[0])
	}
	wantBalcony := domain.LocationProfile{Name: "ベランダ", IsIndoor: false}
	if got[1].ID != balcony.ID || got[1].PlantCount != 0 || got[1].LocationProfile != wantBalcony {
		t.Errorf("balcony = %+v", got[1])
	}

	empty, err := repo.List(ctx, createHousehold(t, pool))
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("empty household: %v, %v (want [] not nil)", empty, err)
	}
}

func TestLocationRepository_HouseholdOf(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewLocationRepository(pool)
	home := createHousehold(t, pool)
	l, err := repo.Create(ctx, home, domain.LocationProfile{Name: "寝室", IsIndoor: true})
	if err != nil {
		t.Fatal(err)
	}

	if got, err := repo.HouseholdOf(ctx, l.ID); err != nil || got != home {
		t.Errorf("HouseholdOf = %s, %v, want %s", got, err, home)
	}
	if _, err := repo.HouseholdOf(ctx, uuid.Must(uuid.NewV7())); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestLocationRepository_Update(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewLocationRepository(pool)
	home := createHousehold(t, pool)
	l, err := repo.Create(ctx, home, domain.LocationProfile{
		Name: "リビング", IsIndoor: true, WindowDirection: domain.DirectionEast, WindowDistance: domain.WindowMiddle, Shading: domain.ShadingNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	createPlant(t, pool, home, l.ID, false)

	// 名前と向きだけを変え、遮光は未設定に戻す。送らなかった距離と室内かどうかは変わらない
	name, south, unset := "リビングの東窓", domain.DirectionSouth, domain.Shading("")
	got, err := repo.Update(ctx, home, l.ID, domain.LocationPatch{Name: &name, WindowDirection: &south, Shading: &unset})
	if err != nil {
		t.Fatal(err)
	}
	want := domain.LocationProfile{Name: name, IsIndoor: true, WindowDirection: south, WindowDistance: domain.WindowMiddle}
	if got.LocationProfile != want || got.PlantCount != 1 {
		t.Errorf("updated = %+v, want %+v with 1 plant", got, want)
	}
	var shading *string
	if err := pool.QueryRow(ctx, `SELECT shading FROM locations WHERE id = $1`, l.ID).Scan(&shading); err != nil || shading != nil {
		t.Errorf("shading = %v, %v, want NULL", shading, err)
	}

	outdoor := false
	if got, err := repo.Update(ctx, home, l.ID, domain.LocationPatch{IsIndoor: &outdoor}); err != nil || got.IsIndoor || got.Name != name {
		t.Errorf("is_indoor: %+v, %v", got, err)
	}

	// ほかの世帯の ID を指定しても、変えられない
	if _, err := repo.Update(ctx, createHousehold(t, pool), l.ID, domain.LocationPatch{Name: &name}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other household: err = %v, want ErrNotFound", err)
	}
}

func TestLocationRepository_Delete(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewLocationRepository(pool)
	home := createHousehold(t, pool)
	l, err := repo.Create(ctx, home, domain.LocationProfile{Name: "玄関", IsIndoor: true})
	if err != nil {
		t.Fatal(err)
	}
	active := createPlant(t, pool, home, l.ID, false)
	archived := createPlant(t, pool, home, l.ID, true)

	if err := repo.Delete(ctx, createHousehold(t, pool), l.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other household: err = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(ctx, home, l.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.HouseholdOf(ctx, l.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("location still exists: %v", err)
	}
	// 植物は消えずに「置き場所未設定」になり、世帯には属したまま（アーカイブした植物も同じ）
	for _, id := range []uuid.UUID{active, archived} {
		var locationID *uuid.UUID
		var householdID uuid.UUID
		if err := pool.QueryRow(ctx, `SELECT location_id, household_id FROM plants WHERE id = $1`, id).Scan(&locationID, &householdID); err != nil {
			t.Fatalf("plant %s: %v", id, err)
		}
		if locationID != nil || householdID != home {
			t.Errorf("plant %s: location_id = %v, household_id = %s", id, locationID, householdID)
		}
	}

	if err := repo.Delete(ctx, home, l.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second delete: err = %v, want ErrNotFound", err)
	}
}

// 環境の項目は、アプリの検証を通らずに書き込まれても、DB の制約で知らない値を拒む。
func TestLocations_EnvironmentChecks(t *testing.T) {
	t.Parallel()
	pool := pgtest.New(t)
	home := createHousehold(t, pool)
	for _, col := range []string{"window_direction", "window_distance", "shading"} {
		_, err := pool.Exec(t.Context(), `INSERT INTO locations (household_id, name, `+col+`) VALUES ($1, 'x', 'unknown')`, home)
		if err == nil {
			t.Errorf("%s: unknown value was accepted", col)
		}
	}
}
