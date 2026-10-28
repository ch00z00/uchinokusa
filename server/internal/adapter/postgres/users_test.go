package postgres_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/pgtest"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func TestUserRepository_GetOrCreate(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	repo := postgres.NewUserRepository(pgtest.New(t))
	identity := domain.Identity{UID: "uid-1", Name: "ゆうすけ", Provider: "apple.com"}

	created, err := repo.GetOrCreate(ctx, identity)
	if err != nil {
		t.Fatalf("first GetOrCreate: %v", err)
	}
	if created.ID.Version() != 7 || created.DisplayName != "ゆうすけ" || created.AuthProvider != "apple.com" {
		t.Errorf("created = %+v", created)
	}

	// 2回目は、名前が変わっていても既存のユーザーを返す（表示名はユーザーが変えたものを保つ）
	again, err := repo.GetOrCreate(ctx, domain.Identity{UID: "uid-1", Name: "別の名前", Provider: "apple.com"})
	if err != nil {
		t.Fatalf("second GetOrCreate: %v", err)
	}
	if again.ID != created.ID || again.DisplayName != "ゆうすけ" {
		t.Errorf("again = %+v, want the existing user %+v", again, created)
	}
}

// 初めてのユーザーのリクエストが同時に届いても、ユーザーは1人だけ作られる。
func TestUserRepository_GetOrCreate_Concurrent(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewUserRepository(pool)

	const n = 10
	ids := make([]uuid.UUID, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			u, err := repo.GetOrCreate(ctx, domain.Identity{UID: "uid-race", Name: "A", Provider: "google.com"})
			ids[i], errs[i] = u.ID, err
		})
	}
	wg.Wait()

	for i := range n {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Errorf("goroutine %d got %s, want %s", i, ids[i], ids[0])
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE firebase_uid = 'uid-race'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("users = %d, want 1", count)
	}
}

// 削除済み（退会した）ユーザーは、トークンがまだ有効でも使えない。
func TestUserRepository_GetOrCreate_DeletedUser(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	pool := pgtest.New(t)
	repo := postgres.NewUserRepository(pool)

	if _, err := repo.GetOrCreate(ctx, domain.Identity{UID: "uid-deleted", Provider: "google.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET deleted_at = now() WHERE firebase_uid = 'uid-deleted'`); err != nil {
		t.Fatal(err)
	}

	_, err := repo.GetOrCreate(ctx, domain.Identity{UID: "uid-deleted", Provider: "google.com"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUserRepository_UpdateDisplayName(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	repo := postgres.NewUserRepository(pgtest.New(t))

	u, err := repo.GetOrCreate(ctx, domain.Identity{UID: "uid-2", Provider: "apple.com"})
	if err != nil {
		t.Fatal(err)
	}
	if u.DisplayName != "" {
		t.Errorf("initial DisplayName = %q, want empty (no name from provider)", u.DisplayName)
	}

	updated, err := repo.UpdateDisplayName(ctx, u.ID, "はなこ")
	if err != nil {
		t.Fatalf("UpdateDisplayName: %v", err)
	}
	if updated.ID != u.ID || updated.DisplayName != "はなこ" {
		t.Errorf("updated = %+v", updated)
	}

	if _, err := repo.UpdateDisplayName(ctx, uuid.Must(uuid.NewV7()), "x"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown user: err = %v, want ErrNotFound", err)
	}
}
