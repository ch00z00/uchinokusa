package postgres_test

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// suggestionFixture は、提案のリポジトリの結合テストの共通の状態である。recordFixture の世帯・植物・タスクを使う。
type suggestionFixture struct {
	recordFixture
	suggestions *postgres.SuggestionRepository
}

func newSuggestionFixture(t *testing.T) suggestionFixture {
	t.Helper()
	f := suggestionFixture{recordFixture: newRecordFixture(t)}
	f.suggestions = postgres.NewSuggestionRepository(f.pool)
	return f
}

// repotting は、モンステラに植え替えのタスクを作る提案（11/1〜11/30 に有効）である。
func (f suggestionFixture) repotting(key string) domain.Suggestion {
	return domain.Suggestion{
		HouseholdID: f.home, PlantID: f.monsteraPlant, ProviderID: "repotting.v1",
		Kind: domain.SuggestionDo, Action: domain.SuggestionCreateTask, TaskKind: domain.CareRepotting,
		TaskSchedule: domain.Schedule{
			Base:     domain.Interval{Value: 12, Unit: domain.IntervalMonth},
			Seasonal: domain.SeasonalIntervals{domain.SeasonWinter: {Value: 18, Unit: domain.IntervalMonth}},
		},
		ProposedDate: nov(20), MessageKey: "repotting.initial", MessageParams: json.RawMessage(`{}`),
		ReasonKey: "repotting.initial.reason", ReasonParams: json.RawMessage(`{"species": "モンステラ"}`),
		ValidFrom: nov(1), ValidUntil: nov(30), DedupeKey: key, Status: domain.SuggestionPending,
	}
}

func (f suggestionFixture) save(s domain.Suggestion) domain.Suggestion {
	f.t.Helper()
	saved, created, err := f.suggestions.Save(f.ctx, s)
	if err != nil || !created {
		f.t.Fatalf("save: created = %v, err = %v", created, err)
	}
	return saved
}

func TestSuggestionRepository_Save(t *testing.T) {
	t.Parallel()

	t.Run("提案を保存し、作るタスクの周期も読み戻せる", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture(t)
		want := f.repotting("k1")
		got := f.save(want)
		if got.ID == uuid.Nil || got.CreatedAt.IsZero() {
			t.Fatalf("saved = %+v", got)
		}
		if got.TaskSchedule.Base != want.TaskSchedule.Base || got.TaskSchedule.Seasonal[domain.SeasonWinter].Value != 18 ||
			got.ProposedDate != nov(20) || got.ValidUntil != nov(30) || got.Status != domain.SuggestionPending ||
			got.ProviderID != "repotting.v1" || string(got.ReasonParams) != `{"species": "モンステラ"}` {
			t.Errorf("saved = %+v", got)
		}
	})

	t.Run("同じ重複防止のキーは、見送ったあとも作り直さない", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture(t)
		first := f.save(f.repotting("k1"))
		if _, err := f.suggestions.Dismiss(f.ctx, f.home, first.ID, f.sakura); err != nil {
			t.Fatal(err)
		}
		if _, created, err := f.suggestions.Save(f.ctx, f.repotting("k1")); err != nil || created {
			t.Errorf("created = %v, err = %v", created, err)
		}
		if n := f.count(`SELECT count(*) FROM suggestions WHERE household_id = $1`, f.home); n != 1 {
			t.Errorf("count = %d", n)
		}
	})

	t.Run("キーの重複は世帯ごとに見る", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture(t)
		f.save(f.repotting("k1"))
		other := f.repotting("k1")
		other.HouseholdID = f.other
		other.PlantID = createPlant(t, f.pool, f.other, f.otherLocation, false)
		f.save(other)
	})
}

func TestSuggestionRepository_Retract(t *testing.T) {
	t.Parallel()
	f := newSuggestionFixture(t)
	warn := func(key, provider string, task uuid.UUID) domain.Suggestion {
		s := f.repotting(key)
		s.Kind, s.Action, s.TaskID, s.ProviderID = domain.SuggestionAvoid, domain.SuggestionInfo, task, provider
		return f.save(s)
	}
	otherTask := f.createTask(f.monsteraPlant, domain.CareTaskDraft{CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareRepotting, Schedule: wateringDraft.Schedule}, NextDueOn: nov(20)})
	old := warn("old", "repotting.v1", f.task.ID)
	kept := warn("kept", "repotting.v1", f.task.ID)
	dismissed := warn("dismissed", "repotting.v1", f.task.ID)
	if _, err := f.suggestions.Dismiss(f.ctx, f.home, dismissed.ID, f.sakura); err != nil {
		t.Fatal(err)
	}
	otherProvider := warn("weather", "weather.v1", f.task.ID)
	onOtherTask := warn("other-task", "repotting.v1", otherTask.ID)

	if err := f.suggestions.Retract(f.ctx, f.home, "repotting.v1", f.task.ID, []string{"kept"}); err != nil {
		t.Fatal(err)
	}
	status := func(id uuid.UUID) string {
		var st string
		f.queryRow(`SELECT status FROM suggestions WHERE id = $1`, id).scan(&st)
		return st
	}
	// 撤回するのは、そのプロバイダの、そのタスクへの未回答の提案で、今回返さなかったものだけ
	for id, want := range map[uuid.UUID]string{old.ID: "expired", kept.ID: "pending", dismissed.ID: "dismissed", otherProvider.ID: "pending", onOtherTask.ID: "pending"} {
		if got := status(id); got != want {
			t.Errorf("%v: status = %s, want %s", id, got, want)
		}
	}

	t.Run("残すキーがなければ、すべて撤回する", func(t *testing.T) {
		if err := f.suggestions.Retract(f.ctx, f.home, "repotting.v1", f.task.ID, nil); err != nil {
			t.Fatal(err)
		}
		if got := status(kept.ID); got != "expired" {
			t.Errorf("status = %s, want expired", got)
		}
	})

	t.Run("撤回した提案は、同じキーで来れば未回答に戻す", func(t *testing.T) {
		s := f.repotting("old")
		s.Kind, s.Action, s.TaskID, s.ValidUntil = domain.SuggestionAvoid, domain.SuggestionInfo, f.task.ID, nov(28)
		got, created, err := f.suggestions.Save(f.ctx, s)
		if err != nil || !created || got.ID != old.ID || got.Status != domain.SuggestionPending || got.ValidUntil != nov(28) {
			t.Errorf("revived = %+v, created = %v, err = %v", got, created, err)
		}
	})

	t.Run("見送った提案は、同じキーで来ても戻さない", func(t *testing.T) {
		s := f.repotting("dismissed")
		s.Kind, s.Action, s.TaskID = domain.SuggestionAvoid, domain.SuggestionInfo, f.task.ID
		if _, created, err := f.suggestions.Save(f.ctx, s); err != nil || created || status(dismissed.ID) != "dismissed" {
			t.Errorf("created = %v, err = %v, status = %s", created, err, status(dismissed.ID))
		}
	})
}

func TestSuggestionRepository_ListActive(t *testing.T) {
	t.Parallel()
	f := newSuggestionFixture(t)
	active := f.save(f.repotting("active"))
	f.save(func() domain.Suggestion { s := f.repotting("later"); s.ValidFrom = nov(10); return s }())
	f.save(func() domain.Suggestion {
		s := f.repotting("expired")
		s.ValidFrom, s.ValidUntil = nov(1), nov(4)
		return s
	}())
	noLimit := f.save(func() domain.Suggestion { s := f.repotting("no-limit"); s.ValidUntil = civil.Date{}; return s }())
	dismissed := f.save(f.repotting("dismissed"))
	if _, err := f.suggestions.Dismiss(f.ctx, f.home, dismissed.ID, f.sakura); err != nil {
		t.Fatal(err)
	}

	got, err := f.suggestions.ListActive(f.ctx, f.home, nov(5))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != active.ID || got[1].ID != noLimit.ID {
		t.Errorf("active = %+v", got)
	}
	if got, err := f.suggestions.ListActive(f.ctx, f.other, nov(5)); err != nil || len(got) != 0 {
		t.Errorf("other household: %+v, %v", got, err)
	}
}

func TestSuggestionRepository_Accept(t *testing.T) {
	t.Parallel()

	t.Run("タスクを作る提案は、変えた日を予定日にしたタスクを作り、対応した人を記録する", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture(t)
		s := f.save(f.repotting("k1"))
		date := "2026-11-25"
		got, task, err := f.suggestions.Accept(f.ctx, f.home, s.ID, f.sakura, domain.AcceptInput{Date: &date}, nov(6))
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != domain.SuggestionAccepted || got.RespondedBy != (uuid.NullUUID{UUID: f.sakura, Valid: true}) || got.RespondedAt.IsZero() {
			t.Errorf("suggestion = %+v", got)
		}
		if task.PlantID != f.monsteraPlant || task.Kind != domain.CareRepotting || task.NextDueOn != nov(25) ||
			task.Schedule.Base.Value != 12 || task.Schedule.Seasonal[domain.SeasonWinter].Value != 18 {
			t.Errorf("task = %+v", task)
		}
	})

	t.Run("同時に採用しても、タスクは1つだけ作る", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture(t)
		s := f.save(f.repotting("k1"))
		var wg sync.WaitGroup
		errs := make([]error, 5)
		for i := range errs {
			wg.Go(func() {
				_, _, errs[i] = f.suggestions.Accept(f.ctx, f.home, s.ID, f.sakura, domain.AcceptInput{}, nov(6))
			})
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			var c *domain.ConflictError
			switch {
			case err == nil:
				ok++
			case !errors.As(err, &c) || c.Code != domain.ConflictAlreadyResponded:
				t.Errorf("err = %v", err)
			}
		}
		if n := f.count(`SELECT count(*) FROM care_tasks WHERE plant_id = $1 AND kind = 'repotting'`, f.monsteraPlant); ok != 1 || n != 1 {
			t.Errorf("accepted = %d, tasks = %d", ok, n)
		}
	})

	t.Run("採用は提案の行ロックを待ってから、答えたかを確かめる", func(t *testing.T) {
		t.Parallel()
		// ロックの前に確かめると、同時の見送り（ここでは手で変える）を見落として、見送られた提案からタスクを作ってしまう
		f := newSuggestionFixture(t)
		s := f.save(f.repotting("k1"))
		tx, err := f.pool.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(f.ctx) }() // コミットしたあとは何もしない
		if _, err := tx.Exec(f.ctx, `UPDATE suggestions SET status = 'dismissed', responded_at = now() WHERE id = $1`, s.ID); err != nil {
			t.Fatal(err)
		}
		accepted := make(chan error, 1)
		go func() {
			_, _, err := f.suggestions.Accept(f.ctx, f.home, s.ID, f.sakura, domain.AcceptInput{}, nov(6))
			accepted <- err
		}()
		time.Sleep(300 * time.Millisecond) // 採用が、ロックを待つところまで進むのを待つ
		if err := tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}

		var c *domain.ConflictError
		if err := <-accepted; !errors.As(err, &c) || c.Code != domain.ConflictAlreadyResponded {
			t.Errorf("err = %v, want already_responded", err)
		}
		if n := f.count(`SELECT count(*) FROM care_tasks WHERE plant_id = $1 AND kind = 'repotting'`, f.monsteraPlant); n != 0 {
			t.Errorf("tasks = %d, want 0", n)
		}
	})

	t.Run("タスクを作らない提案は、対応だけを記録する", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture(t)
		info := f.repotting("k1")
		info.Kind, info.Action, info.TaskID = domain.SuggestionAvoid, domain.SuggestionInfo, f.task.ID
		s := f.save(info)
		got, task, err := f.suggestions.Accept(f.ctx, f.home, s.ID, f.sakura, domain.AcceptInput{}, nov(6))
		if err != nil || got.Status != domain.SuggestionAccepted || task.ID != uuid.Nil || got.TaskID != f.task.ID {
			t.Errorf("suggestion = %+v, task = %+v, err = %v", got, task, err)
		}
	})

	t.Run("ほかの世帯の提案は見つからない", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture(t)
		s := f.save(f.repotting("k1"))
		if _, _, err := f.suggestions.Accept(f.ctx, f.other, s.ID, f.sakura, domain.AcceptInput{}, nov(6)); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestSuggestionRepository_Dismiss(t *testing.T) {
	t.Parallel()
	f := newSuggestionFixture(t)
	s := f.save(f.repotting("k1"))

	got, err := f.suggestions.Dismiss(f.ctx, f.home, s.ID, f.sakura)
	if err != nil || got.Status != domain.SuggestionDismissed || got.RespondedBy.UUID != f.sakura || got.RespondedAt.IsZero() {
		t.Errorf("suggestion = %+v, err = %v", got, err)
	}
	var c *domain.ConflictError
	if _, err := f.suggestions.Dismiss(f.ctx, f.home, s.ID, f.sakura); !errors.As(err, &c) || c.Code != domain.ConflictAlreadyResponded {
		t.Errorf("twice: err = %v", err)
	}
	if _, err := f.suggestions.Dismiss(f.ctx, f.other, s.ID, f.sakura); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other household: err = %v", err)
	}
}

func TestSuggestionRepository_HouseholdOfAndTimeZone(t *testing.T) {
	t.Parallel()
	f := newSuggestionFixture(t)
	s := f.save(f.repotting("k1"))
	if got, err := f.suggestions.HouseholdOf(f.ctx, s.ID); err != nil || got != f.home {
		t.Errorf("household = %v, %v", got, err)
	}
	if _, err := f.suggestions.HouseholdOf(f.ctx, uuid.Must(uuid.NewV7())); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if tz, err := f.suggestions.TimeZoneOf(f.ctx, f.home); err != nil || tz == "" {
		t.Errorf("time zone = %q, %v", tz, err)
	}
}

func TestSuggestionRepository_Subject(t *testing.T) {
	t.Parallel()
	f := newSuggestionFixture(t)
	f.exec(`UPDATE plants SET location_id = $2, acquired_on = '2026-10-20' WHERE id = $1`, f.monsteraPlant, f.living)
	f.exec(`UPDATE households SET region_prefecture = '東京都' WHERE id = $1`, f.home)
	paused := f.createTask(f.monsteraPlant, domain.CareTaskDraft{CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareFertilizing, Schedule: wateringDraft.Schedule}, NextDueOn: nov(10)})
	f.exec(`UPDATE care_tasks SET status = 'paused' WHERE id = $1`, paused.ID)
	deleted := f.createTask(f.monsteraPlant, domain.CareTaskDraft{CareTaskProfile: domain.CareTaskProfile{Kind: domain.CarePruning, Schedule: wateringDraft.Schedule}, NextDueOn: nov(10)})
	f.exec(`UPDATE care_tasks SET deleted_at = now() WHERE id = $1`, deleted.ID)
	f.completeOn(4)
	if _, _, err := f.records.Skip(f.ctx, f.home, f.task.ID, f.sakura, domain.Skip{CycleSeq: 2, Reason: domain.SkipSoilWet, PerformedOn: nov(5), RecheckOn: nov(6)}); err != nil {
		t.Fatal(err)
	}

	got, err := f.suggestions.Subject(f.ctx, f.monsteraPlant)
	if err != nil {
		t.Fatal(err)
	}
	if got.Plant.ID != f.monsteraPlant || got.Plant.AcquiredOn != (civil.Date{Year: 2026, Month: 10, Day: 20}) || got.Household.ID != f.home || got.Household.Region.Prefecture != "東京都" {
		t.Errorf("plant = %+v, household = %+v", got.Plant, got.Household)
	}
	if got.Species == nil || got.Species.ID != f.monstera || got.Location == nil || got.Location.ID != f.living {
		t.Errorf("species = %+v, location = %+v", got.Species, got.Location)
	}
	// 削除したタスクは除き、一時停止中のタスクは含める（予定日の重なりなどを、プロバイダが見られるように）
	if len(got.Tasks) != 2 {
		t.Errorf("tasks = %+v", got.Tasks)
	}
	// 記録は新しい順で、スキップの理由も渡す
	if len(got.Records) != 2 || got.Records[0].SkipReason != domain.SkipSoilWet || got.Records[1].Outcome != domain.OutcomeCompleted {
		t.Errorf("records = %+v", got.Records)
	}

	// 種類と置き場所が未設定なら nil
	bare, err := f.suggestions.Subject(f.ctx, f.pothosPlant)
	if err != nil || bare.Species != nil || bare.Location != nil {
		t.Errorf("bare = %+v, %v", bare, err)
	}
	if _, err := f.suggestions.Subject(f.ctx, uuid.Must(uuid.NewV7())); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
