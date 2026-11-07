// Package ws は、WebSocket でのリアルタイム同期（設計の RealtimeHub）を提供する。
//
// アプリは接続したあと `{"type":"subscribe","householdIds":[...]}` を送り、所属を確かめられた世帯の変更イベントを受け取る。
// 送るのは「何が変わったか」だけで、アプリは受け取ったら REST で取り直す。メッセージの取り決めは docs/decisions/0030 を参照。
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

const (
	// maxSubscriptions は、1本の接続で購読できる世帯の数である。所属できる世帯の数より十分に大きくする。
	maxSubscriptions = 50
	// maxMessageSize は、アプリから受け取るメッセージの最大のバイト数である（世帯の ID が maxSubscriptions 個入る大きさ）。
	maxMessageSize = 4096
	// lookupTimeout は、所属を確かめるのを待つ時間である。
	lookupTimeout = 5 * time.Second
)

// Authorizer は、世帯への所属とロールを確かめる（実装は app.Authorizer）。
type Authorizer interface {
	Require(ctx context.Context, userID, householdID uuid.UUID, action domain.Action) (domain.Membership, error)
}

// Hub は、WebSocket の接続を、購読している世帯ごとにまとめ、変更イベントを配る。
type Hub struct {
	authz  Authorizer
	logger *slog.Logger

	// PingInterval は、接続の生存を確かめる ping の間隔、PingTimeout は、その応答を待つ時間である。
	PingInterval, PingTimeout time.Duration
	// MaxLifetime は、1本の接続を保つ最大の時間である。過ぎたら閉じて、つなぎ直させる（ID トークンの有効期限にそろえる）。
	MaxLifetime time.Duration
	// SendBuffer は、接続ごとに、送り待ちにできるメッセージの数である。あふれたら、その接続を閉じる。
	SendBuffer int

	// ctx は、Shutdown で終わる。すべての接続は、これが終わったら閉じる。
	ctx    context.Context
	cancel context.CancelFunc

	// evalMu は、購読の確認（subscribe）とイベントの配信（Publish）を1つずつ順に行う。
	// 所属を確かめてから購読に入れるまでの間に、メンバーの削除のイベントを配ってしまい、
	// 削除されたメンバーが購読したまま残るのを防ぐ。
	evalMu sync.Mutex
	// mu は、clients と subs、各接続の households を守る。
	mu      sync.Mutex
	clients map[*client]struct{}
	subs    map[uuid.UUID]map[*client]struct{}
}

func NewHub(authz Authorizer, logger *slog.Logger) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		authz: authz, logger: logger,
		PingInterval: 30 * time.Second, PingTimeout: 10 * time.Second, MaxLifetime: time.Hour, SendBuffer: 64,
		ctx: ctx, cancel: cancel,
		clients: map[*client]struct{}{}, subs: map[uuid.UUID]map[*client]struct{}{},
	}
}

// client は、1本の WebSocket の接続である。
type client struct {
	userID uuid.UUID
	send   chan []byte
	// households は、購読している世帯と、購読したときに確かめたロールである（Hub.mu で守る）。
	households map[uuid.UUID]domain.Role

	cancel    context.CancelFunc
	closeOnce sync.Once
	code      websocket.StatusCode
	reason    string
	// abort は、閉じる手続き（相手の応答を待つ）をせずに切るかである。受け取れていない相手には、手続きも届かないため。
	abort bool
}

// close は、接続を code で閉じるよう求める。最初に求めた理由だけを使う。
func (c *client) close(code websocket.StatusCode, reason string) {
	c.closeOnce.Do(func() {
		c.code, c.reason = code, reason
		c.cancel()
	})
}

// kill は、閉じる手続きをせずに接続を切るよう求める。
func (c *client) kill() {
	c.closeOnce.Do(func() {
		c.abort = true
		c.cancel()
	})
}

// enqueue は、メッセージを送り待ちに入れる。あふれたら（アプリが受け取りきれていなければ）、接続を閉じてつなぎ直させる。
// つなぎ直したアプリは、データを取り直すので、取りこぼしから回復できる。
func (c *client) enqueue(msg []byte) {
	select {
	case c.send <- msg:
	default:
		c.kill()
	}
}

// Serve は、リクエストを WebSocket の接続に切り替え、切れるまで送受信する。userID は、認証したユーザーである。
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	// Origin ヘッダーのあるリクエスト（ブラウザ）は、同じオリジンからだけ受け付ける（ライブラリの既定）
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept がエラーのレスポンスを返している
	}
	conn.SetReadLimit(maxMessageSize)

	// 接続を切り替えたあとは r.Context() が使えない（切れても終わらない）ので、Hub の ctx から作る
	ctx, cancel := context.WithTimeout(h.ctx, h.MaxLifetime)
	defer cancel()

	c := &client{userID: userID, send: make(chan []byte, h.SendBuffer), households: map[uuid.UUID]domain.Role{}, cancel: cancel}
	h.add(c)
	defer h.remove(c)

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		h.read(ctx, conn, c)
	}()
	err = h.write(ctx, conn, c)
	// 閉じる理由がまだなければ（保つ時間が過ぎた、またはサーバーを止める）、つなぎ直させる。
	// closeOnce を通すので、これより後に kill・close が呼ばれても、理由は書き換わらない
	c.close(websocket.StatusGoingAway, "reconnect")
	if err != nil || c.abort {
		// 送れない・ping に応えない・受け取りきれていない接続は、閉じる手続きを待たずに切る
		_ = conn.CloseNow()
	} else {
		_ = conn.Close(c.code, c.reason)
	}
	<-readDone
}

// write は、送り待ちのメッセージを送り、定期的に ping を送る。ctx が終わったら nil を、送れなかったらエラーを返す。
func (h *Hub) write(ctx context.Context, conn *websocket.Conn, c *client) error {
	ticker := time.NewTicker(h.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-c.send:
			if err := writeTimeout(ctx, h.PingTimeout, func(ctx context.Context) error { return conn.Write(ctx, websocket.MessageText, msg) }); err != nil {
				return err
			}
		case <-ticker.C:
			if err := writeTimeout(ctx, h.PingTimeout, conn.Ping); err != nil {
				return err
			}
		}
	}
}

func writeTimeout(ctx context.Context, timeout time.Duration, f func(context.Context) error) error {
	// ctx が終わったことによる失敗は、閉じる手続きに任せるため、ctx とは切り離して待つ
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	return f(wctx)
}

// read は、アプリからのメッセージを受け取る。接続が閉じたら戻る。
// ライブラリは、ctx が終わると接続を閉じてしまうので、ctx を渡さずに読み、閉じる手続き（conn.Close）で抜ける。
// 読み続けていることで、ping の応答（pong）も受け取れる。
func (h *Hub) read(ctx context.Context, conn *websocket.Conn, c *client) {
	defer c.cancel()
	for {
		_, data, err := conn.Read(context.Background())
		if err != nil {
			return
		}
		var msg struct {
			Type         string      `json:"type"`
			HouseholdIDs []uuid.UUID `json:"householdIds"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			c.close(websocket.StatusPolicyViolation, "invalid message")
			return
		}
		switch msg.Type {
		case "subscribe":
			h.subscribe(ctx, c, msg.HouseholdIDs)
		case "ping":
			// アプリからの ping（React Native からは WebSocket の ping を送れないため）
			c.enqueue(marshal(map[string]string{"type": "pong"}))
		default:
			// 新しいアプリが送る、このサーバーの知らない種類は、無視する
			h.logger.DebugContext(ctx, "unknown realtime message", slog.String("type", msg.Type))
		}
	}
}

// subscribe は、購読する世帯を ids に置き換える。所属を確かめられた世帯を subscribed で、
// 所属していない世帯を membership_revoked で知らせる。
func (h *Hub) subscribe(ctx context.Context, c *client, ids []uuid.UUID) {
	if len(ids) > maxSubscriptions {
		c.close(websocket.StatusPolicyViolation, "too many households")
		return
	}
	h.evalMu.Lock()
	defer h.evalMu.Unlock()

	granted := map[uuid.UUID]domain.Role{}
	accepted := []uuid.UUID{}
	var revoked []uuid.UUID
	for _, id := range ids {
		if _, ok := granted[id]; ok || slices.Contains(revoked, id) {
			continue
		}
		m, err := h.lookup(ctx, c.userID, id)
		switch {
		case errors.Is(err, domain.ErrNotFound):
			revoked = append(revoked, id)
		case err != nil:
			h.logger.ErrorContext(ctx, "check realtime subscription", slog.String("household_id", id.String()), slog.Any("error", err))
			c.close(websocket.StatusTryAgainLater, "retry")
			return
		default:
			granted[id] = m.Role
			accepted = append(accepted, id)
		}
	}

	h.mu.Lock()
	for id := range c.households {
		h.unsubscribeLocked(c, id)
	}
	for id, role := range granted {
		c.households[id] = role
		if h.subs[id] == nil {
			h.subs[id] = map[*client]struct{}{}
		}
		h.subs[id][c] = struct{}{}
	}
	h.mu.Unlock()

	c.enqueue(marshal(subscribedMessage{Type: "subscribed", HouseholdIDs: accepted}))
	for _, id := range revoked {
		c.enqueue(marshal(householdMessage{Type: "membership_revoked", HouseholdID: id}))
	}
}

// lookup は、ユーザーの世帯でのメンバーシップを確かめる。所属していなければ domain.ErrNotFound を返す。
func (h *Hub) lookup(ctx context.Context, userID, householdID uuid.UUID) (domain.Membership, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	return h.authz.Require(ctx, userID, householdID, domain.ActionRead)
}

// Publish は、変更イベントを、その世帯を購読しているすべての接続に送る（操作した人の接続にも送る。
// アプリはバージョン番号を抜けなく追うため）。ChangeListener から、届いた順に呼ぶ。
// メンバーの変更のときは、変更を受けた本人の接続の購読を見直す（要件4-6、4-7）。
func (h *Hub) Publish(e domain.ChangeEvent) {
	h.evalMu.Lock()
	defer h.evalMu.Unlock()

	h.mu.Lock()
	targets := make([]*client, 0, len(h.subs[e.HouseholdID]))
	for c := range h.subs[e.HouseholdID] {
		targets = append(targets, c)
	}
	h.mu.Unlock()
	if len(targets) == 0 {
		return
	}

	if e.Entity == domain.ChangeMember {
		targets = h.reviewMember(e, targets)
	}
	msg := marshal(toChangedMessage(e))
	for _, c := range targets {
		c.enqueue(msg)
	}
	if e.Entity == domain.ChangeHousehold && e.Op == domain.ChangeDeleted {
		h.mu.Lock()
		for c := range h.subs[e.HouseholdID] {
			h.unsubscribeLocked(c, e.HouseholdID)
		}
		h.mu.Unlock()
	}
}

// reviewMember は、変更を受けたメンバー本人の接続について、所属がなくなっていれば membership_revoked を送って購読をやめさせ、
// ロールが変わっていれば role_changed を送る。変更イベントを送る接続（購読をやめさせた接続を除く）を返す。
func (h *Hub) reviewMember(e domain.ChangeEvent, targets []*client) []*client {
	var affected bool
	for _, c := range targets {
		affected = affected || c.userID == e.EntityID
	}
	if !affected {
		return targets
	}

	var role domain.Role
	revoke := e.Op == domain.ChangeDeleted
	if !revoke {
		m, err := h.lookup(h.ctx, e.EntityID, e.HouseholdID)
		switch {
		case errors.Is(err, domain.ErrNotFound):
			revoke = true
		case err != nil:
			// 確かめられなければ、購読はそのままにする（アプリは、つなぎ直したときに確かめ直される）
			h.logger.Error("check realtime membership", slog.String("household_id", e.HouseholdID.String()), slog.Any("error", err))
			return targets
		default:
			role = m.Role
		}
	}

	out := targets[:0:0]
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range targets {
		if c.userID != e.EntityID {
			out = append(out, c)
			continue
		}
		if revoke {
			h.unsubscribeLocked(c, e.HouseholdID)
			c.enqueue(marshal(householdMessage{Type: "membership_revoked", HouseholdID: e.HouseholdID}))
			continue
		}
		if c.households[e.HouseholdID] != role {
			c.households[e.HouseholdID] = role
			c.enqueue(marshal(roleChangedMessage{Type: "role_changed", HouseholdID: e.HouseholdID, Role: role}))
		}
		out = append(out, c)
	}
	return out
}

// Reset は、すべての接続を閉じて、つなぎ直させる。変更イベントを受け取れなかった間がある
// （ChangeListener がつなぎ直した）ときに呼ぶ。つなぎ直したアプリは、購読を確かめ直し、データを取り直す。
func (h *Hub) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		c.close(websocket.StatusTryAgainLater, "resync")
	}
}

// Shutdown は、すべての接続を閉じる（StatusGoingAway）。以後の接続も、すぐに閉じる。
// http.Server.Shutdown は、WebSocket に切り替えた接続を待たないので、その前に呼ぶ。
func (h *Hub) Shutdown() {
	h.cancel()
}

func (h *Hub) add(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id := range c.households {
		h.unsubscribeLocked(c, id)
	}
	delete(h.clients, c)
}

func (h *Hub) unsubscribeLocked(c *client, householdID uuid.UUID) {
	delete(c.households, householdID)
	delete(h.subs[householdID], c)
	if len(h.subs[householdID]) == 0 {
		delete(h.subs, householdID)
	}
}
