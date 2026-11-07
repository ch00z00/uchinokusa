package postgres_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/job"
)

// reminderFixture は、世帯（自宅）に、11/4 が予定日の水やり（モンステラ）と肥料（ポトス）があり、
// 一時停止中・削除済み・先の予定・アーカイブした植物のタスクもある状態である。
type reminderFixture struct {
	taskFixture
	reminders *postgres.ReminderRepository
	devices   *postgres.DeviceRepository
}

func newReminderFixture(t *testing.T) reminderFixture {
	t.Helper()
	f := reminderFixture{taskFixture: newTaskFixture(t)}
	f.reminders = postgres.NewReminderRepository(f.pool)
	f.devices = postgres.NewDeviceRepository(f.pool)
	draft := func(kind domain.CareKind, due civil.Date) domain.CareTaskDraft {
		d := wateringDraft
		d.Kind, d.NextDueOn = kind, due
		return d
	}
	f.createTask(f.monsteraPlant, draft(domain.CareWatering, nov(4)))
	f.createTask(f.pothosPlant, draft(domain.CareFertilizing, nov(4)))
	f.createTask(f.pothosPlant, draft(domain.CareRepotting, nov(20))) // 先の予定
	paused := f.createTask(f.monsteraPlant, draft(domain.CarePruning, nov(1)))
	if _, err := f.repo.SetStatus(f.ctx, f.home, paused.ID, domain.TaskPaused, nil); err != nil {
		t.Fatal(err)
	}
	deleted := f.createTask(f.pothosPlant, draft(domain.CarePruning, nov(1)))
	if err := f.repo.Delete(f.ctx, f.home, deleted.ID); err != nil {
		t.Fatal(err)
	}
	archived := f.create("枯れたシダ", domain.PlantProfile{})
	f.createTask(archived.ID, draft(domain.CareWatering, nov(1)))
	if _, err := f.plantFixture.repo.SetArchived(f.ctx, f.home, archived.ID, true); err != nil {
		t.Fatal(err)
	}
	return f
}

// member は、世帯のメンバー（role）を作り、メンバーシップの ID を返す。
func (f reminderFixture) member(user uuid.UUID, role string) uuid.UUID {
	f.t.Helper()
	return uuid.MustParse(f.membership(f.home.String(), user.String(), role))
}

// device は、ユーザーの端末を登録し、最後に使った日時を seen にする。
func (f reminderFixture) device(user uuid.UUID, token, tz string, seen time.Time) {
	f.t.Helper()
	reg, err := domain.NewDeviceRegistration(token, "ios", tz)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.devices.Upsert(f.ctx, user, reg); err != nil {
		f.t.Fatal(err)
	}
	f.exec(`UPDATE user_devices SET last_seen_at = $2 WHERE fcm_token = $1`, token, seen)
}

func (f reminderFixture) dispatch(c domain.ReminderCandidate, date civil.Date, sendErr error) (domain.DispatchOutcome, *domain.ReminderBatch) {
	f.t.Helper()
	var got *domain.ReminderBatch
	out, err := f.reminders.Dispatch(f.ctx, c, date, func(_ context.Context, b domain.ReminderBatch) error {
		got = &b
		return sendErr
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return out, got
}

func TestReminderRepository_Candidates(t *testing.T) {
	t.Parallel()
	f := newReminderFixture(t)
	now := time.Now()
	sakura, mom, dad, guest := uuid.MustParse(f.user()), uuid.MustParse(f.user()), uuid.MustParse(f.user()), uuid.MustParse(f.user())
	sakuraM, momM := f.member(sakura, "editor"), f.member(mom, "viewer")
	f.member(dad, "owner")
	f.member(guest, "viewer")
	f.exec(`UPDATE memberships SET reminder_time = '21:30' WHERE id = $1`, momM)
	f.exec(`UPDATE memberships SET reminder_enabled = false WHERE user_id = $1`, dad)
	// さくらは、最後に使った端末（スマホ）のタイムゾーンで判定する
	f.device(sakura, "sakura-tablet", "America/New_York", now.Add(-48*time.Hour))
	f.device(sakura, "sakura-phone", "Asia/Tokyo", now.Add(-time.Hour))
	f.device(mom, "mom-phone", "Europe/London", now)
	f.device(dad, "dad-phone", "Asia/Tokyo", now) // リマインダーを切っている
	// guest は端末がない

	got, err := f.reminders.Candidates(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(got, func(a, b domain.ReminderCandidate) int { return slices.Compare(a.MembershipID[:], b.MembershipID[:]) })
	want := []domain.ReminderCandidate{
		{MembershipID: sakuraM, HouseholdID: f.home, UserID: sakura, ReminderTime: civil.Time{Hour: 8}, TimeZone: "Asia/Tokyo"},
		{MembershipID: momM, HouseholdID: f.home, UserID: mom, ReminderTime: civil.Time{Hour: 21, Minute: 30}, TimeZone: "Europe/London"},
	}
	slices.SortFunc(want, func(a, b domain.ReminderCandidate) int { return slices.Compare(a.MembershipID[:], b.MembershipID[:]) })
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidates = %+v\nwant         %+v", got, want)
	}
}

func TestReminderRepository_Dispatch(t *testing.T) {
	t.Parallel()
	candidate := func(f reminderFixture) domain.ReminderCandidate {
		user := uuid.MustParse(f.user())
		return domain.ReminderCandidate{MembershipID: f.member(user, "viewer"), HouseholdID: f.home, UserID: user, ReminderTime: civil.Time{Hour: 8}, TimeZone: "Asia/Tokyo"}
	}
	dispatches := func(f reminderFixture) int {
		return f.count(`SELECT count(*) FROM reminder_dispatches`)
	}

	t.Run("その日以前の未完了タスクを渡し、送ったら1日1回の記録を残す", func(t *testing.T) {
		t.Parallel()
		f := newReminderFixture(t)
		c := candidate(f)
		out, b := f.dispatch(c, nov(5), nil)
		// 一時停止中・削除済み・先の予定・アーカイブした植物のタスクは入れない。並びは今日やること一覧と同じ（予定日、植物の名前の順）
		want := domain.ReminderBatch{ReminderCandidate: c, Date: nov(5), HouseholdName: "自宅", Items: []domain.ReminderItem{{Kind: domain.CareFertilizing}, {Kind: domain.CareWatering}}}
		if out != domain.DispatchSent || b == nil || !reflect.DeepEqual(*b, want) {
			t.Fatalf("outcome = %v, batch = %+v", out, b)
		}
		var date string
		f.queryRow(`SELECT local_date::text FROM reminder_dispatches WHERE membership_id = $1`, c.MembershipID).scan(&date)
		if date != "2026-11-05" {
			t.Errorf("local_date = %s", date)
		}

		// 同じ日に二重に実行しても、送らない（要件15-2）
		if out, b := f.dispatch(c, nov(5), nil); out != domain.DispatchSkipped || b != nil {
			t.Errorf("again: outcome = %v, batch = %v", out, b)
		}
		// 次の日は送る
		if out, _ := f.dispatch(c, nov(6), nil); out != domain.DispatchSent {
			t.Errorf("next day: outcome = %v", out)
		}
	})

	t.Run("予定日がその日のタスクも入れ、その他は作業名を渡す", func(t *testing.T) {
		t.Parallel()
		f := newReminderFixture(t)
		d := wateringDraft
		d.Kind, d.CustomName, d.NextDueOn = domain.CareOther, "葉水", nov(3)
		f.createTask(f.monsteraPlant, d)
		// 11/3 が予定日なのは、葉水だけ
		_, b := f.dispatch(candidate(f), nov(3), nil)
		if b == nil || !reflect.DeepEqual(b.Items, []domain.ReminderItem{{Kind: domain.CareOther, CustomName: "葉水"}}) {
			t.Errorf("batch = %+v", b)
		}
	})

	t.Run("未完了タスクがなければ送らず、記録も残さない（要件15-4）", func(t *testing.T) {
		t.Parallel()
		f := newReminderFixture(t)
		c := candidate(f)
		// 11/3 には、まだ予定日のタスクがない（すべて完了・スキップすると予定日が先に進むのと同じ）
		if out, b := f.dispatch(c, nov(3), nil); out != domain.DispatchSkipped || b != nil {
			t.Errorf("outcome = %v, batch = %v", out, b)
		}
		if n := dispatches(f); n != 0 {
			t.Errorf("dispatches = %d", n)
		}
	})

	t.Run("送れなければ記録を残さず、次の実行で送り直す", func(t *testing.T) {
		t.Parallel()
		f := newReminderFixture(t)
		c := candidate(f)
		if out, _ := f.dispatch(c, nov(5), errors.New("fcm is down")); out != domain.DispatchFailed {
			t.Errorf("outcome = %v", out)
		}
		if n := dispatches(f); n != 0 {
			t.Errorf("dispatches = %d", n)
		}
		if out, _ := f.dispatch(c, nov(5), nil); out != domain.DispatchSent {
			t.Errorf("retry: outcome = %v", out)
		}
	})

	t.Run("対象を選んだあとでリマインダーを切った・世帯を抜けたメンバーには送らない", func(t *testing.T) {
		t.Parallel()
		f := newReminderFixture(t)
		off, left := candidate(f), candidate(f)
		f.exec(`UPDATE memberships SET reminder_enabled = false WHERE id = $1`, off.MembershipID)
		f.exec(`DELETE FROM memberships WHERE id = $1`, left.MembershipID)
		for _, c := range []domain.ReminderCandidate{off, left} {
			if out, b := f.dispatch(c, nov(5), nil); out != domain.DispatchSkipped || b != nil {
				t.Errorf("outcome = %v, batch = %v", out, b)
			}
		}
	})
}

// TestReminderFlow は、タイムゾーンの異なる端末を持つメンバーに、それぞれの時刻に1回だけリマインダーが届くことを確かめる（要件15）。
func TestReminderFlow(t *testing.T) {
	t.Parallel()
	f := newReminderFixture(t)
	sakura, mom, grandma, dad := uuid.MustParse(f.user()), uuid.MustParse(f.user()), uuid.MustParse(f.user()), uuid.MustParse(f.user())
	f.member(sakura, "editor")
	f.member(mom, "owner")
	f.member(grandma, "viewer") // 閲覧者にも送る（要件15-8）
	f.member(dad, "editor")
	f.exec(`UPDATE memberships SET reminder_enabled = false WHERE user_id = $1`, dad)
	seen := time.Now()
	f.device(sakura, "sakura-phone", "Asia/Tokyo", seen)
	f.device(mom, "mom-phone", "America/Los_Angeles", seen) // 旅行中
	f.device(grandma, "grandma-phone", "Asia/Tokyo", seen)
	f.device(dad, "dad-phone", "Asia/Tokyo", seen)

	sender := &recordingSender{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	run := func(now time.Time) job.Result {
		t.Helper()
		push := app.NewPushService(f.devices, sender, logger)
		res, err := job.NewReminders(f.reminders, push, func() time.Time { return now }, logger).Run(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	tokensSent := func() []string {
		var out []string
		for _, s := range sender.sent {
			out = append(out, s.tokens...)
		}
		slices.Sort(out)
		return out
	}

	// 11/5 8:05（東京）＝ 11/4 15:05（ロサンゼルス）：東京の2人に送る
	tokyoMorning := time.Date(2026, 11, 4, 23, 5, 0, 0, time.UTC)
	if res := run(tokyoMorning); res != (job.Result{Processed: 2}) {
		t.Errorf("tokyo: result = %+v", res)
	}
	if got := tokensSent(); !slices.Equal(got, []string{"grandma-phone", "sakura-phone"}) {
		t.Errorf("tokyo: tokens = %v", got)
	}
	want := domain.PushMessage{Title: "自宅", Body: "今日のお手入れが2件あります（肥料1件、水やり1件）", Data: map[string]string{"type": "reminder", "householdId": f.home.String()}}
	if len(sender.sent) == 0 {
		t.Fatal("nothing sent")
	}
	if !reflect.DeepEqual(sender.sent[0].msg, want) {
		t.Errorf("message = %+v", sender.sent[0].msg)
	}

	// 5分後の実行（二重の実行を含む）では、送り直さない
	run(tokyoMorning.Add(5 * time.Minute))
	run(tokyoMorning.Add(5 * time.Minute))
	if n := len(sender.sent); n != 2 {
		t.Errorf("after rerun: sent = %d", n)
	}

	// 11/5 8:05（ロサンゼルス）：母に送る
	if res := run(time.Date(2026, 11, 5, 16, 5, 0, 0, time.UTC)); res != (job.Result{Processed: 1}) {
		t.Errorf("los angeles: result = %+v", res)
	}
	if got := tokensSent(); !slices.Equal(got, []string{"grandma-phone", "mom-phone", "sakura-phone"}) {
		t.Errorf("los angeles: tokens = %v", got)
	}
	var date string
	f.queryRow(`SELECT d.local_date::text FROM reminder_dispatches d JOIN memberships m ON m.id = d.membership_id WHERE m.user_id = $1`, mom).scan(&date)
	if date != "2026-11-05" {
		t.Errorf("mom's local_date = %s", date)
	}
}
