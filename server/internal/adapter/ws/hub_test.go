package ws_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/ws"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

var (
	alice = uuid.MustParse("01920000-0000-7000-8000-00000000000a")
	bob   = uuid.MustParse("01920000-0000-7000-8000-00000000000b")
	home  = uuid.MustParse("01920000-0000-7000-8000-0000000000a1")
	away  = uuid.MustParse("01920000-0000-7000-8000-0000000000a2")
	plant = uuid.MustParse("01920000-0000-7000-8000-0000000000c1")
)

// fakeAuthz は、ユーザーと世帯の組ごとのロールで判定する。
type fakeAuthz struct {
	mu    sync.Mutex
	roles map[[2]uuid.UUID]domain.Role
	err   error
	// held が nil でなければ、判定の結果を決めたあと、held が閉じるまで返さない（確認の途中に割り込むテストのため）
	held    chan struct{}
	entered chan struct{}
}

func newFakeAuthz() *fakeAuthz {
	return &fakeAuthz{roles: map[[2]uuid.UUID]domain.Role{}, entered: make(chan struct{}, 10)}
}

func (f *fakeAuthz) set(user, household uuid.UUID, role domain.Role) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.roles[[2]uuid.UUID{user, household}] = role
}

func (f *fakeAuthz) remove(user, household uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.roles, [2]uuid.UUID{user, household})
}

func (f *fakeAuthz) Require(_ context.Context, user, household uuid.UUID, action domain.Action) (domain.Membership, error) {
	f.mu.Lock()
	role, ok := f.roles[[2]uuid.UUID{user, household}]
	err, held := f.err, f.held
	f.mu.Unlock()
	select {
	case f.entered <- struct{}{}:
	default:
	}
	if held != nil {
		<-held
	}
	switch {
	case err != nil:
		return domain.Membership{}, err
	case !ok:
		return domain.Membership{}, domain.ErrNotFound
	case !role.Allows(action):
		return domain.Membership{}, domain.ErrForbidden
	}
	return domain.Membership{HouseholdID: household, UserID: user, Role: role}, nil
}

type harness struct {
	t     *testing.T
	hub   *ws.Hub
	authz *fakeAuthz
	url   string
}

// newHarness は、Hub を WebSocket のサーバーで公開する。ユーザーは X-User ヘッダーで渡す（認証は httpapi の役目）。
func newHarness(t *testing.T, configure ...func(*ws.Hub)) *harness {
	t.Helper()
	authz := newFakeAuthz()
	hub := ws.NewHub(authz, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, c := range configure {
		c(hub)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.Serve(w, r, uuid.MustParse(r.Header.Get("X-User")))
	}))
	t.Cleanup(func() {
		hub.Shutdown()
		srv.Close()
	})
	return &harness{t: t, hub: hub, authz: authz, url: "ws" + strings.TrimPrefix(srv.URL, "http")}
}

func (h *harness) connect(user uuid.UUID) *websocket.Conn {
	h.t.Helper()
	conn, _, err := websocket.Dial(h.t.Context(), h.url, &websocket.DialOptions{HTTPHeader: http.Header{"X-User": {user.String()}}})
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

// peer は、受け取ったメッセージを順に並べる、アプリ側の接続である。
type peer struct {
	t      *testing.T
	conn   *websocket.Conn
	msgs   chan map[string]any
	closed chan websocket.StatusCode
}

func (h *harness) dial(user uuid.UUID) *peer {
	h.t.Helper()
	p := &peer{t: h.t, conn: h.connect(user), msgs: make(chan map[string]any, 100), closed: make(chan websocket.StatusCode, 1)}
	go func() {
		for {
			_, data, err := p.conn.Read(context.Background())
			if err != nil {
				p.closed <- websocket.CloseStatus(err)
				return
			}
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				panic(err)
			}
			p.msgs <- m
		}
	}()
	return p
}

func (p *peer) send(v any) {
	p.t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		p.t.Fatal(err)
	}
	if err := p.conn.Write(p.t.Context(), websocket.MessageText, data); err != nil {
		p.t.Fatal(err)
	}
}

// expect は、次に届くメッセージが want であることを確かめる。
func (p *peer) expect(want map[string]any) {
	p.t.Helper()
	select {
	case got := <-p.msgs:
		if !reflect.DeepEqual(got, want) {
			p.t.Errorf("message = %v\nwant      %v", got, want)
		}
	case <-time.After(2 * time.Second):
		p.t.Fatalf("no message, want %v", want)
	}
}

// none は、しばらく待ってもメッセージが届かないことを確かめる。
func (p *peer) none() {
	p.t.Helper()
	select {
	case got := <-p.msgs:
		p.t.Errorf("unexpected message %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

// closedWith は、サーバーが接続を code で閉じたことを確かめる。
func (p *peer) closedWith(code websocket.StatusCode) {
	p.t.Helper()
	select {
	case got := <-p.closed:
		if got != code {
			p.t.Errorf("close status = %v, want %v", got, code)
		}
	case <-time.After(3 * time.Second):
		p.t.Fatalf("not closed, want %v", code)
	}
}

func ids(list ...uuid.UUID) []any {
	out := make([]any, 0, len(list))
	for _, id := range list {
		out = append(out, id.String())
	}
	return out
}

// subscribe は、世帯を購読し、購読できた世帯の知らせを確かめる。
func (p *peer) subscribe(want []uuid.UUID, households ...uuid.UUID) {
	p.t.Helper()
	p.send(map[string]any{"type": "subscribe", "householdIds": households})
	p.expect(map[string]any{"type": "subscribed", "householdIds": ids(want...)})
}

func revoked(household uuid.UUID) map[string]any {
	return map[string]any{"type": "membership_revoked", "householdId": household.String()}
}

func plantCreated(household uuid.UUID, version int64) domain.ChangeEvent {
	return domain.ChangeEvent{HouseholdID: household, Version: version, Entity: domain.ChangePlant, EntityID: plant, Op: domain.ChangeCreated, ActorUserID: alice, PlantID: plant}
}

func changed(e domain.ChangeEvent) map[string]any {
	m := map[string]any{"type": "changed", "householdId": e.HouseholdID.String(), "version": float64(e.Version),
		"entity": string(e.Entity), "entityId": e.EntityID.String(), "op": string(e.Op)}
	if e.ActorUserID != uuid.Nil {
		m["actorUserId"] = e.ActorUserID.String()
	}
	if e.PlantID != uuid.Nil {
		m["plantId"] = e.PlantID.String()
	}
	return m
}

func memberChange(user uuid.UUID, op domain.ChangeOp, version int64) domain.ChangeEvent {
	return domain.ChangeEvent{HouseholdID: home, Version: version, Entity: domain.ChangeMember, EntityID: user, Op: op, ActorUserID: alice}
}

func TestHub_Subscribe(t *testing.T) {
	t.Parallel()

	t.Run("所属する世帯だけを購読し、所属しない世帯は membership_revoked で知らせる", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.authz.set(alice, home, domain.RoleViewer)
		p := h.dial(alice)
		p.subscribe([]uuid.UUID{home}, home, away)
		p.expect(revoked(away))

		h.hub.Publish(plantCreated(away, 1))
		h.hub.Publish(plantCreated(home, 1))
		p.expect(changed(plantCreated(home, 1)))
		p.none()
	})

	t.Run("どの世帯にも所属していなければ、空の一覧を返す", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		p := h.dial(alice)
		p.subscribe([]uuid.UUID{}, home)
		p.expect(revoked(home))
	})

	t.Run("購読は、送るたびに置き換える", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.authz.set(alice, home, domain.RoleEditor)
		h.authz.set(alice, away, domain.RoleEditor)
		p := h.dial(alice)
		p.subscribe([]uuid.UUID{home, away}, home, away)
		p.subscribe([]uuid.UUID{away}, away, away)

		h.hub.Publish(plantCreated(home, 1))
		h.hub.Publish(plantCreated(away, 1))
		p.expect(changed(plantCreated(away, 1)))
		p.none()
	})

	t.Run("世帯が多すぎれば、接続を閉じる", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		p := h.dial(alice)
		many := make([]uuid.UUID, 51)
		for i := range many {
			many[i] = uuid.New()
		}
		p.send(map[string]any{"type": "subscribe", "householdIds": many})
		p.closedWith(websocket.StatusPolicyViolation)
	})

	t.Run("読めないメッセージは、接続を閉じる", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		p := h.dial(alice)
		if err := p.conn.Write(t.Context(), websocket.MessageText, []byte("{")); err != nil {
			t.Fatal(err)
		}
		p.closedWith(websocket.StatusPolicyViolation)
	})

	t.Run("知らない種類のメッセージは、無視する", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.authz.set(alice, home, domain.RoleViewer)
		p := h.dial(alice)
		p.send(map[string]any{"type": "hello"})
		p.subscribe([]uuid.UUID{home}, home)
	})

	t.Run("所属を確かめられなければ、つなぎ直させる", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.authz.err = errors.New("database is down")
		p := h.dial(alice)
		p.send(map[string]any{"type": "subscribe", "householdIds": []uuid.UUID{home}})
		p.closedWith(websocket.StatusTryAgainLater)
	})
}

func TestHub_Publish(t *testing.T) {
	t.Parallel()

	t.Run("世帯を購読しているすべての接続に、操作した人の接続にも届ける", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.authz.set(alice, home, domain.RoleOwner)
		h.authz.set(bob, home, domain.RoleEditor)
		h.authz.set(bob, away, domain.RoleEditor)
		a, b, b2 := h.dial(alice), h.dial(bob), h.dial(bob)
		a.subscribe([]uuid.UUID{home}, home)
		b.subscribe([]uuid.UUID{home}, home)
		b2.subscribe([]uuid.UUID{away}, away)

		e := plantCreated(home, 7)
		h.hub.Publish(e)
		a.expect(changed(e))
		b.expect(changed(e))
		b2.none()
	})

	t.Run("操作した人・植物がなければ、その項目を入れない", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.authz.set(alice, home, domain.RoleOwner)
		p := h.dial(alice)
		p.subscribe([]uuid.UUID{home}, home)

		e := domain.ChangeEvent{HouseholdID: home, Version: 3, Entity: domain.ChangeLocation, EntityID: plant, Op: domain.ChangeDeleted}
		h.hub.Publish(e)
		p.expect(map[string]any{"type": "changed", "householdId": home.String(), "version": float64(3), "entity": "location", "entityId": plant.String(), "op": "deleted"})
	})

	t.Run("世帯が削除されたら、知らせたあと購読をやめる", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.authz.set(alice, home, domain.RoleOwner)
		p := h.dial(alice)
		p.subscribe([]uuid.UUID{home}, home)

		e := domain.ChangeEvent{HouseholdID: home, Version: 9, Entity: domain.ChangeHousehold, EntityID: home, Op: domain.ChangeDeleted, ActorUserID: alice}
		h.hub.Publish(e)
		p.expect(changed(e))
		h.hub.Publish(plantCreated(home, 10))
		p.none()
	})
}

func TestHub_MembershipChanges(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (*harness, *peer, *peer) {
		h := newHarness(t)
		h.authz.set(alice, home, domain.RoleOwner)
		h.authz.set(bob, home, domain.RoleEditor)
		a, b := h.dial(alice), h.dial(bob)
		a.subscribe([]uuid.UUID{home}, home)
		b.subscribe([]uuid.UUID{home}, home)
		return h, a, b
	}

	t.Run("削除されたメンバーの接続には membership_revoked を送り、購読をやめる（要件4-7）", func(t *testing.T) {
		t.Parallel()
		h, a, b := setup(t)
		h.authz.remove(bob, home)
		e := memberChange(bob, domain.ChangeDeleted, 5)
		h.hub.Publish(e)
		b.expect(revoked(home))
		a.expect(changed(e))

		h.hub.Publish(plantCreated(home, 6))
		a.expect(changed(plantCreated(home, 6)))
		b.none()
	})

	t.Run("ロールが変わったメンバーの接続には role_changed を送る（要件4-6）", func(t *testing.T) {
		t.Parallel()
		h, a, b := setup(t)
		h.authz.set(bob, home, domain.RoleViewer)
		e := memberChange(bob, domain.ChangeUpdated, 5)
		h.hub.Publish(e)
		b.expect(map[string]any{"type": "role_changed", "householdId": home.String(), "role": "viewer"})
		b.expect(changed(e))
		a.expect(changed(e))

		// 表示名の変更など、ロールの変わらない更新では送らない
		again := memberChange(bob, domain.ChangeUpdated, 6)
		h.hub.Publish(again)
		b.expect(changed(again))
		a.expect(changed(again))
	})

	t.Run("更新でも、所属がなくなっていれば membership_revoked を送る", func(t *testing.T) {
		t.Parallel()
		h, a, b := setup(t)
		h.authz.remove(bob, home)
		e := memberChange(bob, domain.ChangeUpdated, 5)
		h.hub.Publish(e)
		b.expect(revoked(home))
		a.expect(changed(e))
		b.none()
	})

	t.Run("購読の確認の途中に届いた削除も、取りこぼさない", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.authz.set(bob, home, domain.RoleEditor)
		held := make(chan struct{})
		h.authz.held = held
		b := h.dial(bob)
		// 所属を確かめた（メンバーだった）あとで止めておき、その間に削除を届ける
		b.send(map[string]any{"type": "subscribe", "householdIds": []uuid.UUID{home}})
		<-h.authz.entered
		h.authz.remove(bob, home)
		published := make(chan struct{})
		go func() {
			h.hub.Publish(memberChange(bob, domain.ChangeDeleted, 5))
			close(published)
		}()
		select {
		case <-published:
		case <-time.After(100 * time.Millisecond):
		}
		close(held)
		b.expect(map[string]any{"type": "subscribed", "householdIds": ids(home)})
		b.expect(revoked(home))
	})
}

func TestHub_Connections(t *testing.T) {
	t.Parallel()

	waitConnections := func(t *testing.T, h *harness, want int) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for h.hub.Connections() != want {
			if time.Now().After(deadline) {
				t.Fatalf("connections = %d, want %d", h.hub.Connections(), want)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	fastPing := func(hub *ws.Hub) { hub.PingInterval, hub.PingTimeout = 20*time.Millisecond, 50*time.Millisecond }

	t.Run("ping に応えない接続は片付ける", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, fastPing)
		h.connect(alice) // 読まないので、ping に応えない
		waitConnections(t, h, 1)
		waitConnections(t, h, 0)
	})

	t.Run("ping に応える接続は保つ", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, fastPing)
		h.authz.set(alice, home, domain.RoleViewer)
		p := h.dial(alice)
		p.subscribe([]uuid.UUID{home}, home)
		time.Sleep(300 * time.Millisecond)
		h.hub.Publish(plantCreated(home, 1))
		p.expect(changed(plantCreated(home, 1)))
	})

	t.Run("アプリの ping には pong を返す", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		p := h.dial(alice)
		p.send(map[string]any{"type": "ping"})
		p.expect(map[string]any{"type": "pong"})
	})

	t.Run("切れた接続は、購読とあわせて片付ける", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.authz.set(alice, home, domain.RoleViewer)
		p := h.dial(alice)
		p.subscribe([]uuid.UUID{home}, home)
		if err := p.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
			t.Fatal(err)
		}
		waitConnections(t, h, 0)
		if n := h.hub.Subscribers(home); n != 0 {
			t.Errorf("subscribers = %d", n)
		}
	})

	t.Run("送り待ちがあふれた接続は、つなぎ直させる", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(hub *ws.Hub) { hub.SendBuffer = 1 })
		h.authz.set(alice, home, domain.RoleViewer)
		conn := h.connect(alice) // 読まないので、送り待ちがたまる
		data, _ := json.Marshal(map[string]any{"type": "subscribe", "householdIds": []uuid.UUID{home}})
		if err := conn.Write(t.Context(), websocket.MessageText, data); err != nil {
			t.Fatal(err)
		}
		<-h.authz.entered
		waitConnections(t, h, 1)
		for i := range 20000 {
			h.hub.Publish(plantCreated(home, int64(i+1)))
		}
		waitConnections(t, h, 0)
	})

	t.Run("保つ時間を過ぎた接続は、つなぎ直させる", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(hub *ws.Hub) { hub.MaxLifetime = 100 * time.Millisecond })
		h.dial(alice).closedWith(websocket.StatusGoingAway)
		waitConnections(t, h, 0)
	})

	t.Run("Reset は、すべての接続をつなぎ直させる", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		a, b := h.dial(alice), h.dial(bob)
		waitConnections(t, h, 2)
		h.hub.Reset()
		a.closedWith(websocket.StatusTryAgainLater)
		b.closedWith(websocket.StatusTryAgainLater)
	})

	t.Run("Shutdown は、すべての接続を閉じる", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		p := h.dial(alice)
		waitConnections(t, h, 1)
		h.hub.Shutdown()
		p.closedWith(websocket.StatusGoingAway)
	})
}
