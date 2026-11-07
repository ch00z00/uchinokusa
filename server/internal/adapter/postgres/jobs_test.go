package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/pgtest"
)

func TestJobLocker(t *testing.T) {
	t.Parallel()
	pool := pgtest.New(t)
	// 別のインスタンスの代わりに、別のロッカー（プールの別の接続）を使う
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, b := postgres.NewJobLocker(pool, logger), postgres.NewJobLocker(pool, logger)

	unlock, ok, err := a.TryLock(t.Context(), "reminders")
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v", ok, err)
	}
	// 同じジョブは、ほかで実行中なので取れない。取れなかったときは、借りた接続をすぐに返す（何度試しても、プールが尽きない）
	for range 20 {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		_, ok, err := b.TryLock(ctx, "reminders")
		cancel()
		if err != nil || ok {
			t.Fatalf("same job: ok = %v, err = %v", ok, err)
		}
	}
	// 別のジョブは取れる
	other, ok, err := b.TryLock(t.Context(), "completion-notifications")
	if err != nil || !ok {
		t.Errorf("other job: ok = %v, err = %v", ok, err)
	} else {
		other()
	}

	// 外したら、ほかでも取れる（プールに戻した接続にロックが残らない）
	unlock()
	again, ok, err := b.TryLock(t.Context(), "reminders")
	if err != nil || !ok {
		t.Fatalf("after unlock: ok = %v, err = %v", ok, err)
	}
	again()
	// プールに戻した接続に、ロックが残っていない（同じ接続ならロックを取り直せてしまうので、pg_locks で確かめる）
	var held int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND database = (SELECT oid FROM pg_database WHERE datname = current_database())`).Scan(&held); err != nil {
		t.Fatal(err)
	}
	if held != 0 {
		t.Errorf("advisory locks = %d", held)
	}
}
