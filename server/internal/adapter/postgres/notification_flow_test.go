package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/job"
)

// recordingSender は、FCM の代わりに、送った通知を記録する。
type recordingSender struct {
	sent []sentPush
}

type sentPush struct {
	tokens []string
	msg    domain.PushMessage
}

func (s *recordingSender) Send(_ context.Context, tokens []string, msg domain.PushMessage) ([]domain.PushOutcome, error) {
	s.sent = append(s.sent, sentPush{tokens, msg})
	out := make([]domain.PushOutcome, len(tokens))
	for i := range out {
		out[i] = domain.PushDelivered
	}
	return out, nil
}

// TestNotificationFlow は、完了の記録から、まとめた完了通知がほかのメンバーの端末に届くまでを確かめる（要件16）。
func TestNotificationFlow(t *testing.T) {
	t.Parallel()
	f := newOutboxFixture(t)
	devices := postgres.NewDeviceRepository(f.pool)
	for user, token := range map[uuid.UUID]string{f.sakura: "sakura-phone", f.mom: "mom-phone", f.grandma: "grandma-phone", f.dad: "dad-phone"} {
		reg, err := domain.NewDeviceRegistration(token, "ios", "Asia/Tokyo")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := devices.Upsert(f.ctx, user, reg); err != nil {
			t.Fatal(err)
		}
	}
	sender := &recordingSender{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	run := func(now time.Time) job.Result {
		t.Helper()
		push := app.NewPushService(devices, sender, logger)
		res, err := job.NewNotifications(postgres.NewOutboxRepository(f.pool), push, func() time.Time { return now }, logger).Run(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	// さくらが続けて2件完了する
	f.completeTask(f.task)
	f.completeTask(f.fertilizing)

	// 最後の完了から60秒たつまでは、まとめるために待つ
	if res := run(time.Now()); res != (job.Result{}) || len(sender.sent) != 0 {
		t.Fatalf("before quiet: result = %+v, sent = %v", res, sender.sent)
	}

	// 60秒たったら、1件にまとめて、さくら（本人）と父（完了通知を切っている）以外の端末に送る
	if res := run(time.Now().Add(2 * time.Minute)); res != (job.Result{Processed: 1}) {
		t.Errorf("result = %+v", res)
	}
	want := []sentPush{{
		tokens: []string{"grandma-phone", "mom-phone"},
		msg: domain.PushMessage{
			Title: "自宅", Body: "さくらさんが2件のお手入れをしました（水やり1件、肥料1件）",
			Data: map[string]string{"type": "completion", "householdId": f.home.String()},
		},
	}}
	if !reflect.DeepEqual(sender.sent, want) {
		t.Errorf("sent = %+v\nwant   %+v", sender.sent, want)
	}

	// 二重に実行しても、送り直さない
	if res := run(time.Now().Add(3 * time.Minute)); res != (job.Result{}) || len(sender.sent) != 1 {
		t.Errorf("again: result = %+v, sent = %d", res, len(sender.sent))
	}
}
