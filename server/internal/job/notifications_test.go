package job_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/job"
)

var (
	nHome     = uuid.MustParse("01920000-0000-7000-8000-0000000000f1")
	nSakura   = uuid.MustParse("01920000-0000-7000-8000-0000000000f2")
	nMom      = uuid.MustParse("01920000-0000-7000-8000-0000000000f3")
	nMonstera = uuid.MustParse("01920000-0000-7000-8000-0000000000f4")
	nNow      = time.Date(2026, 11, 7, 8, 0, 0, 0, time.UTC)
)

// fakeOutbox は、まとまりごとに決めた内容（batches）を send に渡す。
type fakeOutbox struct {
	groups        []domain.NoticeGroup
	batches       map[domain.NoticeGroup]domain.NoticeBatch
	readyErr      error
	dispatchErr   error
	quiet, oldest time.Time
	outcomes      map[domain.NoticeGroup]domain.DispatchOutcome
}

func (f *fakeOutbox) ReadyGroups(_ context.Context, quietBefore, oldestBefore time.Time, _ int) ([]domain.NoticeGroup, error) {
	f.quiet, f.oldest = quietBefore, oldestBefore
	return f.groups, f.readyErr
}

func (f *fakeOutbox) Dispatch(ctx context.Context, g domain.NoticeGroup, send func(context.Context, domain.NoticeBatch) error) (domain.DispatchOutcome, error) {
	if f.dispatchErr != nil {
		return "", f.dispatchErr
	}
	b, ok := f.batches[g]
	out := domain.DispatchSkipped
	if ok {
		out = domain.DispatchSent
		if err := send(ctx, b); err != nil {
			out = domain.DispatchFailed
		}
	}
	if f.outcomes == nil {
		f.outcomes = map[domain.NoticeGroup]domain.DispatchOutcome{}
	}
	f.outcomes[g] = out
	return out, nil
}

type pushed struct {
	to  []uuid.UUID
	msg domain.PushMessage
}

type fakePusher struct {
	sent []pushed
	err  error
}

func (f *fakePusher) Notify(_ context.Context, userIDs []uuid.UUID, msg domain.PushMessage) (domain.PushReport, error) {
	if f.err != nil {
		return domain.PushReport{}, f.err
	}
	f.sent = append(f.sent, pushed{userIDs, msg})
	return domain.PushReport{Delivered: len(userIDs)}, nil
}

func newNotifications(outbox *fakeOutbox, push *fakePusher) *job.Notifications {
	return job.NewNotifications(outbox, push, func() time.Time { return nNow }, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestNotifications_Run(t *testing.T) {
	t.Parallel()
	completion := domain.NoticeGroup{HouseholdID: nHome, Kind: domain.NoticeCompletion, ActorUserID: nSakura}
	joined := domain.NoticeGroup{HouseholdID: nHome, Kind: domain.NoticeMemberJoined, ActorUserID: nMom}
	undone := domain.NoticeGroup{HouseholdID: nHome, Kind: domain.NoticeCompletion, ActorUserID: nMom}
	items := []domain.CompletionItem{{PlantID: nMonstera, PlantName: "モンステラ", Kind: domain.CareWatering}}
	batches := map[domain.NoticeGroup]domain.NoticeBatch{
		completion: {NoticeGroup: completion, HouseholdName: "自宅", ActorName: "さくら", Items: items, Recipients: []uuid.UUID{nMom}},
		joined:     {NoticeGroup: joined, HouseholdName: "自宅", ActorName: "母", Recipients: []uuid.UUID{nSakura}},
	}

	t.Run("送ってよいまとまりごとに、種類に応じた文面で宛先に送る", func(t *testing.T) {
		t.Parallel()
		outbox := &fakeOutbox{groups: []domain.NoticeGroup{completion, undone, joined}, batches: batches}
		push := &fakePusher{}
		res, err := newNotifications(outbox, push).Run(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		// 取り消された記録だけのまとまり（undone）は送らないので、数えない
		if res != (job.Result{Processed: 2}) {
			t.Errorf("result = %+v", res)
		}
		// 最後の完了から60秒、最初の完了から10分
		if !outbox.quiet.Equal(nNow.Add(-60*time.Second)) || !outbox.oldest.Equal(nNow.Add(-10*time.Minute)) {
			t.Errorf("cutoffs = %v, %v", outbox.quiet, outbox.oldest)
		}
		want := []pushed{
			{[]uuid.UUID{nMom}, domain.CompletionNotice(nHome, "自宅", "さくら", items)},
			{[]uuid.UUID{nSakura}, domain.MemberJoinedNotice(nHome, "自宅", "母")},
		}
		if !reflect.DeepEqual(push.sent, want) {
			t.Errorf("sent = %+v\nwant   %+v", push.sent, want)
		}
	})

	t.Run("送れなかったまとまりは失敗として数え、ほかのまとまりは送る", func(t *testing.T) {
		t.Parallel()
		outbox := &fakeOutbox{groups: []domain.NoticeGroup{completion, joined}, batches: batches}
		res, err := newNotifications(outbox, &fakePusher{err: errors.New("fcm is down")}).Run(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if res != (job.Result{Failed: 2}) || outbox.outcomes[joined] != domain.DispatchFailed {
			t.Errorf("result = %+v, outcomes = %v", res, outbox.outcomes)
		}
	})

	t.Run("アウトボックスを読めなければエラーを返す", func(t *testing.T) {
		t.Parallel()
		for _, outbox := range []*fakeOutbox{
			{readyErr: errors.New("db down")},
			{groups: []domain.NoticeGroup{completion}, dispatchErr: errors.New("db down")},
		} {
			if _, err := newNotifications(outbox, &fakePusher{}).Run(t.Context()); err == nil {
				t.Error("err = nil")
			}
		}
	})
}
