package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres/sqlcgen"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// changesChannel は、変更イベントを知らせる、PostgreSQL の通知のチャンネルである。
const changesChannel = "household_changes"

// changes は、1つのトランザクションでの変更イベントを集める。
type changes struct {
	events []domain.ChangeEvent
}

// add は、変更イベントを足す。plantID は、植物に属するデータのときだけ渡す（ほかは uuid.Nil）。
func (c *changes) add(householdID uuid.UUID, entity domain.ChangeEntity, id uuid.UUID, op domain.ChangeOp, plantID uuid.UUID) {
	c.events = append(c.events, domain.ChangeEvent{HouseholdID: householdID, Entity: entity, EntityID: id, Op: op, PlantID: plantID})
}

// withChanges は、fn をトランザクションで実行し、fn が集めた変更イベントを、コミットの直前に発行する（設計の ChangeBroadcaster）。
// fn がエラーを返せば、ロールバックし、イベントも発行しない。
func withChanges(ctx context.Context, pool *pgxpool.Pool, fn func(*sqlcgen.Queries, *changes) error) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		var ch changes
		if err := fn(q, &ch); err != nil {
			return err
		}
		return publishChanges(ctx, q, ch.events)
	})
}

// publishChanges は、トランザクションの中で、イベントごとに世帯のバージョン番号を1つ進め、pg_notify で知らせる。
// pg_notify の通知は、コミットしたときにだけ届く（ロールバックすれば届かない）ので、「確定したあと」に知らせることになる。
// 操作した人は、context から読む（app.WithActor）。
func publishChanges(ctx context.Context, q *sqlcgen.Queries, events []domain.ChangeEvent) error {
	actor := app.ActorFrom(ctx)
	for _, e := range events {
		version, err := q.BumpHouseholdVersion(ctx, e.HouseholdID)
		if err != nil {
			return fmt.Errorf("bump household version: %w", err)
		}
		e.Version, e.ActorUserID = version, actor
		payload, err := json.Marshal(toChangePayload(e))
		if err != nil {
			return fmt.Errorf("marshal change: %w", err)
		}
		if err := q.NotifyHouseholdChange(ctx, string(payload)); err != nil {
			return fmt.Errorf("notify change: %w", err)
		}
	}
	return nil
}

// changePayload は、通知の中身である。項目の名前は、アプリに送るメッセージ（設計の RealtimeMessage）にそろえる。
// 通知の中身は 8000 バイトまでなので、データそのものは入れず、ID だけを入れる。
type changePayload struct {
	HouseholdID uuid.UUID  `json:"householdId"`
	Version     int64      `json:"version"`
	Entity      string     `json:"entity"`
	EntityID    uuid.UUID  `json:"entityId"`
	Op          string     `json:"op"`
	ActorUserID *uuid.UUID `json:"actorUserId,omitempty"`
	PlantID     *uuid.UUID `json:"plantId,omitempty"`
}

func toChangePayload(e domain.ChangeEvent) changePayload {
	p := changePayload{HouseholdID: e.HouseholdID, Version: e.Version, Entity: string(e.Entity), EntityID: e.EntityID, Op: string(e.Op)}
	if e.ActorUserID != uuid.Nil {
		p.ActorUserID = &e.ActorUserID
	}
	if e.PlantID != uuid.Nil {
		p.PlantID = &e.PlantID
	}
	return p
}

func decodeChange(payload string) (domain.ChangeEvent, error) {
	var p changePayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return domain.ChangeEvent{}, err
	}
	if p.HouseholdID == uuid.Nil || p.Entity == "" || p.Op == "" {
		return domain.ChangeEvent{}, errors.New("missing fields")
	}
	e := domain.ChangeEvent{HouseholdID: p.HouseholdID, Version: p.Version, Entity: domain.ChangeEntity(p.Entity), EntityID: p.EntityID, Op: domain.ChangeOp(p.Op)}
	if p.ActorUserID != nil {
		e.ActorUserID = *p.ActorUserID
	}
	if p.PlantID != nil {
		e.PlantID = *p.PlantID
	}
	return e, nil
}

// ChangeListener は、変更イベントの通知を受け取る（設計の RealtimeHub の受け口）。
// 専用の接続で LISTEN するので、どの API インスタンスで起きた変更も、すべてのインスタンスが受け取れる。
// 接続が切れたら、待つ時間を延ばしながらつなぎ直す。切れている間の通知は失われる（配信は「最大1回」。
// アプリは、再接続とバージョン番号の飛びで取り直す）。
type ChangeListener struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
	// OnListen は、LISTEN を始める（つなぎ直す）たびに呼ばれる。テストで、受け取れる状態になるのを待つために使う。
	OnListen func()
	// MinBackoff・MaxBackoff は、つなぎ直すまでに待つ時間の最初と上限である。
	MinBackoff, MaxBackoff time.Duration
}

func NewChangeListener(pool *pgxpool.Pool, logger *slog.Logger) *ChangeListener {
	return &ChangeListener{pool: pool, logger: logger, MinBackoff: time.Second, MaxBackoff: 30 * time.Second}
}

// Run は、ctx が終わるまで通知を受け取り、イベントごとに handle を呼ぶ。読めない通知は、ログに残して飛ばす。
// handle は受け取りの処理を止めるので、時間のかかる処理は handle の中でほかの goroutine に渡す。
func (l *ChangeListener) Run(ctx context.Context, handle func(domain.ChangeEvent)) error {
	backoff := l.MinBackoff
	for {
		err := l.listen(ctx, handle, func() { backoff = l.MinBackoff })
		if ctx.Err() != nil {
			return ctx.Err()
		}
		l.logger.WarnContext(ctx, "change listener disconnected", "error", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, l.MaxBackoff)
	}
}

// listen は、1本の接続で LISTEN し、接続が切れるか ctx が終わるまで通知を受け取る。
func (l *ChangeListener) listen(ctx context.Context, handle func(domain.ChangeEvent), connected func()) error {
	pc, err := l.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire: %w", err)
	}
	// プールから切り離して、この接続を占有する（LISTEN した接続を、ほかのクエリに使わせないため）
	conn := pc.Hijack()
	defer conn.Close(context.WithoutCancel(ctx))

	if _, err := conn.Exec(ctx, "LISTEN "+changesChannel); err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	connected()
	if l.OnListen != nil {
		l.OnListen()
	}
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait for notification: %w", err)
		}
		e, err := decodeChange(n.Payload)
		if err != nil {
			l.logger.ErrorContext(ctx, "decode change", "error", err)
			continue
		}
		handle(e)
	}
}
