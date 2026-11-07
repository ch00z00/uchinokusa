package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

const (
	// NoticeQuiet は、完了をまとめる時間である。最後の完了からこれだけたったら送る（続けて完了している間は待つ。要件16-4）。
	NoticeQuiet = 60 * time.Second
	// NoticeMaxDelay は、まとめるために待つ最大の時間である。最初の完了からこれだけたったら、続けて完了していても送る。
	NoticeMaxDelay = 10 * time.Minute
	// noticeGroupsPerRun は、1回の実行で送るまとまりの数の上限である。残りは次の実行で送る。
	noticeGroupsPerRun = 100
)

// Outbox は、通知のアウトボックスである（実装は postgres.OutboxRepository）。
type Outbox interface {
	// ReadyGroups は、送ってよいまとまりを返す。最後の行が quietBefore 以前か、最初の行が oldestBefore 以前のまとまりである。
	ReadyGroups(ctx context.Context, quietBefore, oldestBefore time.Time, limit int) ([]domain.NoticeGroup, error)
	// Dispatch は、まとまりの未送信の行をロックし、送る内容を send に渡す。send が nil を返せば送信済みにし、
	// エラーを返せば送り直すよう試行回数を増やす。送るものがなければ send を呼ばない。
	Dispatch(ctx context.Context, g domain.NoticeGroup, send func(context.Context, domain.NoticeBatch) error) (domain.DispatchOutcome, error)
}

// Pusher は、ユーザーたちの端末にプッシュ通知を送る（実装は app.PushService）。
type Pusher interface {
	Notify(ctx context.Context, userIDs []uuid.UUID, msg domain.PushMessage) (domain.PushReport, error)
}

// Notifications は、アウトボックスの完了通知・参加の通知を、まとめて送るジョブである（要件16、3-8）。
type Notifications struct {
	outbox Outbox
	push   Pusher
	now    func() time.Time
	logger *slog.Logger
}

func NewNotifications(outbox Outbox, push Pusher, now func() time.Time, logger *slog.Logger) *Notifications {
	return &Notifications{outbox: outbox, push: push, now: now, logger: logger}
}

func (n *Notifications) Name() string { return "notifications" }

// Run は、送ってよいまとまりを1件ずつ送る。FCM に送れなかったまとまりは失敗として数え、次の実行で送り直す。
// アウトボックスを読み書きできなければ、そこで止めてエラーを返す（Cloud Scheduler がやり直す）。
func (n *Notifications) Run(ctx context.Context) (Result, error) {
	var res Result
	now := n.now()
	groups, err := n.outbox.ReadyGroups(ctx, now.Add(-NoticeQuiet), now.Add(-NoticeMaxDelay), noticeGroupsPerRun)
	if err != nil {
		return res, fmt.Errorf("ready notice groups: %w", err)
	}
	for _, g := range groups {
		out, err := n.outbox.Dispatch(ctx, g, n.send)
		if err != nil {
			return res, fmt.Errorf("dispatch notice: %w", err)
		}
		switch out {
		case domain.DispatchSent:
			res.Processed++
		case domain.DispatchFailed:
			res.Failed++
		}
	}
	return res, nil
}

// send は、まとまりの種類に応じた文面を作り、宛先の端末に送る。
func (n *Notifications) send(ctx context.Context, b domain.NoticeBatch) error {
	var msg domain.PushMessage
	switch b.Kind {
	case domain.NoticeCompletion:
		msg = domain.CompletionNotice(b.HouseholdID, b.HouseholdName, b.ActorName, b.Items)
	case domain.NoticeMemberJoined:
		msg = domain.MemberJoinedNotice(b.HouseholdID, b.HouseholdName, b.ActorName)
	default:
		return fmt.Errorf("unknown notice kind %q", b.Kind)
	}
	if _, err := n.push.Notify(ctx, b.Recipients, msg); err != nil {
		n.logger.WarnContext(ctx, "send notice", slog.String("household_id", b.HouseholdID.String()), slog.String("kind", string(b.Kind)), slog.Any("error", err))
		return err
	}
	return nil
}
