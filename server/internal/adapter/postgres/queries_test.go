package postgres_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/pgtest"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
)

// sqlc で生成したコードが、マイグレーションのスキーマに対して動くことを確かめる。
func TestUserQueries(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	q := sqlcgen.New(pgtest.New(t))

	created, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
		FirebaseUID:  "uid-1",
		DisplayName:  "ゆうすけ",
		AuthProvider: "apple.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.ID.Version() != 7 {
		t.Errorf("ID version = %d, want 7", created.ID.Version())
	}

	got, err := q.GetUserByFirebaseUID(ctx, "uid-1")
	if err != nil {
		t.Fatalf("GetUserByFirebaseUID: %v", err)
	}
	if got.ID != created.ID || got.DisplayName != "ゆうすけ" {
		t.Errorf("got %+v, want %+v", got, created)
	}

	if _, err := q.GetUserByFirebaseUID(ctx, "unknown"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("unknown uid: err = %v, want pgx.ErrNoRows", err)
	}
}
