package job_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/job"
)

// fakeReminderStore は、メンバーごとに決めた内容（items）があれば send に渡す。
type fakeReminderStore struct {
	candidates  []domain.ReminderCandidate
	items       map[uuid.UUID][]domain.ReminderItem // メンバーシップごとの未完了タスク
	candErr     error
	dispatchErr error
	dates       map[uuid.UUID]civil.Date // Dispatch に渡された日付
}

func (f *fakeReminderStore) Candidates(context.Context) ([]domain.ReminderCandidate, error) {
	return f.candidates, f.candErr
}

func (f *fakeReminderStore) Dispatch(ctx context.Context, c domain.ReminderCandidate, date civil.Date, send func(context.Context, domain.ReminderBatch) error) (domain.DispatchOutcome, error) {
	if f.dispatchErr != nil {
		return "", f.dispatchErr
	}
	if f.dates == nil {
		f.dates = map[uuid.UUID]civil.Date{}
	}
	f.dates[c.MembershipID] = date
	items, ok := f.items[c.MembershipID]
	if !ok {
		return domain.DispatchSkipped, nil
	}
	if err := send(ctx, domain.ReminderBatch{ReminderCandidate: c, Date: date, HouseholdName: "自宅", Items: items}); err != nil {
		return domain.DispatchFailed, nil
	}
	return domain.DispatchSent, nil
}

func TestReminders_Run(t *testing.T) {
	t.Parallel()
	// 2026-11-08 8:10（東京）。ニューヨークでは 11-07 18:10
	now := time.Date(2026, 11, 7, 23, 10, 0, 0, time.UTC)
	cand := func(user uuid.UUID, at civil.Time, tz string) domain.ReminderCandidate {
		return domain.ReminderCandidate{MembershipID: uuid.New(), HouseholdID: nHome, UserID: user, ReminderTime: at, TimeZone: tz}
	}
	eight := civil.Time{Hour: 8}
	sakura := cand(nSakura, eight, "Asia/Tokyo")
	momNY := cand(nMom, eight, "America/New_York")              // まだ時刻の前
	late := cand(uuid.New(), civil.Time{Hour: 6}, "Asia/Tokyo") // 時刻から1時間を過ぎた
	// 読めないタイムゾーン。UTC とみなすと時刻の間（23:00 から1時間）に入ってしまう
	broken := cand(uuid.New(), civil.Time{Hour: 23}, "Mars/Olympus")
	grandma := cand(uuid.New(), eight, "Asia/Tokyo")
	nothing := cand(uuid.New(), eight, "Asia/Tokyo") // 未完了タスクがない
	items := []domain.ReminderItem{{Kind: domain.CareWatering}, {Kind: domain.CareFertilizing}}
	run := func(store *fakeReminderStore, push *fakePusher) (job.Result, error) {
		return job.NewReminders(store, push, func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil))).Run(t.Context())
	}

	t.Run("端末のタイムゾーンで時刻になったメンバーに、その日の日付で送る", func(t *testing.T) {
		t.Parallel()
		store := &fakeReminderStore{candidates: []domain.ReminderCandidate{sakura, momNY, late, broken, nothing, grandma}, items: map[uuid.UUID][]domain.ReminderItem{sakura.MembershipID: items, grandma.MembershipID: items}}
		push := &fakePusher{}
		res, err := run(store, push)
		if err != nil {
			t.Fatal(err)
		}
		if res != (job.Result{Processed: 2}) {
			t.Errorf("result = %+v", res)
		}
		// 時刻の前・過ぎたメンバーと、読めないタイムゾーンのメンバーは、確かめもしない
		nov8 := civil.Date{Year: 2026, Month: 11, Day: 8}
		wantDates := map[uuid.UUID]civil.Date{sakura.MembershipID: nov8, nothing.MembershipID: nov8, grandma.MembershipID: nov8}
		if !reflect.DeepEqual(store.dates, wantDates) {
			t.Errorf("dates = %v", store.dates)
		}
		want := []pushed{{[]uuid.UUID{nSakura}, domain.ReminderNotice(nHome, "自宅", items)}, {[]uuid.UUID{grandma.UserID}, domain.ReminderNotice(nHome, "自宅", items)}}
		if !reflect.DeepEqual(push.sent, want) {
			t.Errorf("sent = %+v", push.sent)
		}
	})

	t.Run("送れなかったメンバーは、失敗として数える", func(t *testing.T) {
		t.Parallel()
		store := &fakeReminderStore{candidates: []domain.ReminderCandidate{sakura}, items: map[uuid.UUID][]domain.ReminderItem{sakura.MembershipID: items}}
		res, err := run(store, &fakePusher{err: errors.New("fcm is down")})
		if err != nil || res != (job.Result{Failed: 1}) {
			t.Errorf("result = %+v, err = %v", res, err)
		}
	})

	t.Run("読み書きできなければエラーを返す", func(t *testing.T) {
		t.Parallel()
		for _, store := range []*fakeReminderStore{
			{candErr: errors.New("db down")},
			{candidates: []domain.ReminderCandidate{sakura}, dispatchErr: errors.New("db down")},
		} {
			if _, err := run(store, &fakePusher{}); err == nil {
				t.Error("err = nil")
			}
		}
	})
}
