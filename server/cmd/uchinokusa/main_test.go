package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/pgtest"
)

func envFrom(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

var validEnv = map[string]string{"DATABASE_URL": "postgres://localhost:5432/uchinokusa"}

func TestRun_Usage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "サブコマンドがない", args: nil},
		{name: "知らないサブコマンド", args: []string{"unknown"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := run(context.Background(), tt.args, envFrom(validEnv), io.Discard)
			if !errors.Is(err, errUsage) {
				t.Errorf("run() error = %v, want errUsage", err)
			}
		})
	}
}

func TestRun_InvalidConfig(t *testing.T) {
	t.Parallel()

	err := run(context.Background(), []string{"serve"}, envFrom(map[string]string{}), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("run() error = %v, want config error about DATABASE_URL", err)
	}
}

// seed は、何度実行しても同じ結果になる（植物マスタの行が重ならない）。
func TestRun_Seed(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	dbURL := pgtest.NewEmptyURL(t)
	env := envFrom(map[string]string{"DATABASE_URL": dbURL})

	if err := run(ctx, []string{"migrate"}, env, io.Discard); err != nil {
		t.Fatalf("run(migrate) error = %v", err)
	}
	for i := range 2 {
		if err := run(ctx, []string{"seed"}, env, io.Discard); err != nil {
			t.Fatalf("run(seed) #%d error = %v", i+1, err)
		}
	}

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var total, distinct int
	if err := conn.QueryRow(ctx, `SELECT count(*), count(DISTINCT slug) FROM species`).Scan(&total, &distinct); err != nil {
		t.Fatal(err)
	}
	if total < 30 || total != distinct {
		t.Errorf("species: total = %d, distinct slugs = %d, want at least 30 and no duplicates", total, distinct)
	}
}

func TestRun_Migrate(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	dbURL := pgtest.NewEmptyURL(t)

	if err := run(ctx, []string{"migrate"}, envFrom(map[string]string{"DATABASE_URL": dbURL}), io.Discard); err != nil {
		t.Fatalf("run(migrate) error = %v", err)
	}

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT to_regclass('public.care_records') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("care_records table does not exist after migrate")
	}
}
