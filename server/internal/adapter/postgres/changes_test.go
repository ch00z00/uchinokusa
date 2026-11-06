package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// changeWatcher は、テストのデータベースの変更イベントを、ChangeListener で受け取る。
type changeWatcher struct {
	t         *testing.T
	events    chan domain.ChangeEvent
	listening chan struct{}
}

func watchChanges(t *testing.T, pool *pgxpool.Pool) changeWatcher {
	t.Helper()
	w := changeWatcher{t: t, events: make(chan domain.ChangeEvent, 100), listening: make(chan struct{}, 10)}
	l := postgres.NewChangeListener(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	l.MinBackoff, l.MaxBackoff = 10*time.Millisecond, 50*time.Millisecond
	l.OnListen = func() { w.listening <- struct{}{} }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = l.Run(ctx, func(e domain.ChangeEvent) { w.events <- e })
	}()
	t.Cleanup(func() { cancel(); <-done })
	w.waitListening()
	return w
}

func (w changeWatcher) waitListening() {
	w.t.Helper()
	select {
	case <-w.listening:
	case <-time.After(5 * time.Second):
		w.t.Fatal("listener did not start")
	}
}

// collect は、届いたイベントを、しばらく届かなくなるまで集める。
func (w changeWatcher) collect() []domain.ChangeEvent {
	w.t.Helper()
	var out []domain.ChangeEvent
	for {
		select {
		case e := <-w.events:
			out = append(out, e)
		case <-time.After(300 * time.Millisecond):
			return out
		}
	}
}

func TestChanges_PublishedOnCommit(t *testing.T) {
	t.Parallel()
	f := newPlantFixture(t)
	w := watchChanges(t, f.pool)
	locations := postgres.NewLocationRepository(f.pool)
	actor := uuid.Must(uuid.NewV7())
	ctx := app.WithActor(f.ctx, actor)

	first, err := locations.Create(ctx, f.home, domain.LocationProfile{Name: "玄関", IsIndoor: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := locations.Update(ctx, f.home, first.ID, domain.LocationPatch{Name: ptrTo("玄関の棚")}); err != nil {
		t.Fatal(err)
	}

	got := w.collect()
	want := []domain.ChangeEvent{
		{HouseholdID: f.home, Version: 1, Entity: domain.ChangeLocation, EntityID: first.ID, Op: domain.ChangeCreated, ActorUserID: actor},
		{HouseholdID: f.home, Version: 2, Entity: domain.ChangeLocation, EntityID: first.ID, Op: domain.ChangeUpdated, ActorUserID: actor},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("events = %+v\nwant %+v", got, want)
	}
	if n := f.count(`SELECT version FROM households WHERE id = $1`, f.home); n != 2 {
		t.Errorf("version = %d, want 2", n)
	}
}

func TestChanges_NotPublishedOnRollback(t *testing.T) {
	t.Parallel()
	f := newPlantFixture(t)
	w := watchChanges(t, f.pool)

	// ほかの世帯の置き場所を指定すると、外部キーの違反でロールバックする
	if _, err := f.repo.Create(f.ctx, f.home, domain.PlantProfile{Name: "パキラ", LocationID: f.otherLocation}); err == nil {
		t.Fatal("err = nil")
	}
	if got := w.collect(); len(got) != 0 {
		t.Errorf("events = %+v", got)
	}
	if n := f.count(`SELECT version FROM households WHERE id = $1`, f.home); n != 0 {
		t.Errorf("version = %d, want 0", n)
	}
}

func TestChangeListener_Reconnects(t *testing.T) {
	t.Parallel()
	f := newPlantFixture(t)
	w := watchChanges(t, f.pool)

	// LISTEN している接続を、データベースの側から切る
	f.exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = current_database() AND query = 'LISTEN household_changes'`)
	w.waitListening()

	loc, err := postgres.NewLocationRepository(f.pool).Create(f.ctx, f.home, domain.LocationProfile{Name: "玄関", IsIndoor: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.collect(); len(got) != 1 || got[0].EntityID != loc.ID {
		t.Errorf("events after reconnect = %+v", got)
	}
}

func TestChangeListener_SkipsMalformedPayload(t *testing.T) {
	t.Parallel()
	f := newPlantFixture(t)
	w := watchChanges(t, f.pool)

	f.exec(`SELECT pg_notify('household_changes', 'not json')`)
	f.exec(`SELECT pg_notify('household_changes', '{"version": 1}')`)
	loc, err := postgres.NewLocationRepository(f.pool).Create(f.ctx, f.home, domain.LocationProfile{Name: "玄関", IsIndoor: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.collect(); len(got) != 1 || got[0].EntityID != loc.ID {
		t.Errorf("events = %+v", got)
	}
}
