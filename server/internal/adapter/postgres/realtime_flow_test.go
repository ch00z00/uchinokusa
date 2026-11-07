package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/adapter/ws"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// TestRealtimeFlow は、完了の記録が、pg_notify → ChangeListener → Hub を通って、
// 同じ世帯を購読している別の接続に届くまでを確かめる（要件11-1）。メンバーの削除で購読が止まることも確かめる（要件4-7）。
func TestRealtimeFlow(t *testing.T) {
	t.Parallel()
	f := newRecordFixture(t)
	mom := uuid.MustParse(f.user())
	f.membership(f.home.String(), f.sakura.String(), "owner")
	f.membership(f.home.String(), mom.String(), "viewer")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := ws.NewHub(app.NewAuthorizer(postgres.NewMembershipRepository(f.pool)), logger)
	listening := make(chan struct{}, 10)
	listener := postgres.NewChangeListener(f.pool, logger)
	listener.OnListen = func() {
		hub.Reset()
		listening <- struct{}{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = listener.Run(ctx, hub.Publish)
	}()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.Serve(w, r, uuid.MustParse(r.Header.Get("X-User")))
	}))
	t.Cleanup(func() {
		hub.Shutdown()
		srv.Close()
		cancel()
		<-done
	})
	select {
	case <-listening:
	case <-time.After(5 * time.Second):
		t.Fatal("listener did not start")
	}

	// お母さんの端末が、世帯を購読する
	conn, _, err := websocket.Dial(t.Context(), "ws"+strings.TrimPrefix(srv.URL, "http"), &websocket.DialOptions{HTTPHeader: http.Header{"X-User": {mom.String()}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	next := func() map[string]any {
		t.Helper()
		rctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_, data, err := conn.Read(rctx)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	sub, _ := json.Marshal(map[string]any{"type": "subscribe", "householdIds": []uuid.UUID{f.home}})
	if err := conn.Write(t.Context(), websocket.MessageText, sub); err != nil {
		t.Fatal(err)
	}
	if m := next(); m["type"] != "subscribed" {
		t.Fatalf("message = %v", m)
	}

	// さくらさんが、別の API（リポジトリ）で完了する
	rec, _, err := f.records.Complete(app.WithActor(f.ctx, f.sakura), f.home, f.task.ID, f.sakura, domain.Completion{CycleSeq: 1, PerformedOn: performedOn})
	if err != nil {
		t.Fatal(err)
	}
	record, task := next(), next()
	for _, c := range []struct {
		got            map[string]any
		entity, id, op string
	}{
		{record, "record", rec.ID.String(), "created"},
		{task, "task", f.task.ID.String(), "updated"},
	} {
		if c.got["type"] != "changed" || c.got["householdId"] != f.home.String() || c.got["entity"] != c.entity || c.got["entityId"] != c.id ||
			c.got["op"] != c.op || c.got["actorUserId"] != f.sakura.String() || c.got["plantId"] != f.monsteraPlant.String() {
			t.Errorf("message = %v", c.got)
		}
	}
	if record["version"].(float64)+1 != task["version"].(float64) {
		t.Errorf("versions = %v, %v", record["version"], task["version"])
	}

	// オーナーがお母さんを世帯から外すと、購読が止まる
	if err := postgres.NewMemberRepository(f.pool).Remove(app.WithActor(f.ctx, f.sakura), f.home, mom); err != nil {
		t.Fatal(err)
	}
	if m := next(); m["type"] != "membership_revoked" || m["householdId"] != f.home.String() {
		t.Errorf("message = %v", m)
	}
}
