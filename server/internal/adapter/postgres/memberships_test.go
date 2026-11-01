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

// createHousehold は、テスト用の世帯を作り、その ID を返す（世帯のリポジトリはタスク 4.2 で作る）。
func createHousehold(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(t.Context(), `INSERT INTO households (name) VALUES ('自宅') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func addMember(t *testing.T, pool *pgxpool.Pool, householdID, userID uuid.UUID, role domain.Role) {
	t.Helper()
	if _, err := pool.Exec(t.Context(), `INSERT INTO memberships (household_id, user_id, role) VALUES ($1, $2, $3)`, householdID, userID, role); err != nil {
		t.Fatal(err)
	}
}

func TestMembershipRepository_Get(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewMembershipRepository(pool)

	home := createHousehold(t, pool)
	parents := createHousehold(t, pool)
	alice := createUser(t, pool, "uid-alice")
	addMember(t, pool, home, alice, domain.RoleOwner)
	addMember(t, pool, parents, alice, domain.RoleViewer) // 世帯ごとに異なるロールで所属できる（要件2-3）

	tests := []struct {
		name        string
		householdID uuid.UUID
		want        domain.Role
	}{
		{name: "自宅ではオーナー", householdID: home, want: domain.RoleOwner},
		{name: "実家では閲覧者", householdID: parents, want: domain.RoleViewer},
	}
	for _, tt := range tests {
		m, err := repo.Get(ctx, tt.householdID, alice)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		want := domain.Membership{HouseholdID: tt.householdID, UserID: alice, Role: tt.want}
		if m != want {
			t.Errorf("%s: got %+v, want %+v", tt.name, m, want)
		}
	}

	bob := createUser(t, pool, "uid-bob")
	if _, err := repo.Get(ctx, home, bob); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("non-member: err = %v, want ErrNotFound", err)
	}
	if _, err := repo.Get(ctx, uuid.Must(uuid.NewV7()), alice); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown household: err = %v, want ErrNotFound", err)
	}
}
