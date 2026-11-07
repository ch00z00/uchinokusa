package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// ReminderStore は、リマインダーの対象と送信の記録である（実装は postgres.ReminderRepository）。
type ReminderStore interface {
	// Candidates は、リマインダーが有効で、端末があるメンバーを返す。
	Candidates(ctx context.Context) ([]domain.ReminderCandidate, error)
	// Dispatch は、メンバーの世帯に date 以前の未完了タスクがあり、その日にまだ送っていなければ、送る内容を send に渡す。
	// send が nil を返せば、その日の送信を記録する（1日1回。要件15-2）。エラーを返せば記録せず、次の実行で送り直す。
	Dispatch(ctx context.Context, c domain.ReminderCandidate, date civil.Date, send func(context.Context, domain.ReminderBatch) error) (domain.DispatchOutcome, error)
}

// Reminders は、メンバーごとに、設定した時刻にリマインダーを送るジョブである（要件15）。
type Reminders struct {
	store  ReminderStore
	push   Pusher
	now    func() time.Time
	logger *slog.Logger
}

func NewReminders(store ReminderStore, push Pusher, now func() time.Time, logger *slog.Logger) *Reminders {
	return &Reminders{store: store, push: push, now: now, logger: logger}
}

func (r *Reminders) Name() string { return "reminders" }

// Run は、端末のタイムゾーンでリマインダーの時刻から1時間の間にいるメンバーに、リマインダーを送る。
// 送れなかったメンバーは失敗として数え、1時間の間の次の実行で送り直す。
func (r *Reminders) Run(ctx context.Context) (Result, error) {
	var res Result
	candidates, err := r.store.Candidates(ctx)
	if err != nil {
		return res, fmt.Errorf("reminder candidates: %w", err)
	}
	now := r.now()
	for _, c := range candidates {
		loc, err := time.LoadLocation(c.TimeZone)
		if err != nil {
			// 端末の登録で確かめているので、ふつうは起きない（タイムゾーンのデータから消えた名前など）
			r.logger.WarnContext(ctx, "unknown device time zone", slog.String("time_zone", c.TimeZone))
			continue
		}
		date, ok := domain.ReminderDate(now, loc, c.ReminderTime)
		if !ok {
			continue
		}
		out, err := r.store.Dispatch(ctx, c, date, r.send)
		if err != nil {
			return res, fmt.Errorf("dispatch reminder: %w", err)
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

func (r *Reminders) send(ctx context.Context, b domain.ReminderBatch) error {
	msg := domain.ReminderNotice(b.HouseholdID, b.HouseholdName, b.Items)
	if _, err := r.push.Notify(ctx, []uuid.UUID{b.UserID}, msg); err != nil {
		r.logger.WarnContext(ctx, "send reminder", slog.String("household_id", b.HouseholdID.String()), slog.Any("error", err))
		return err
	}
	return nil
}
