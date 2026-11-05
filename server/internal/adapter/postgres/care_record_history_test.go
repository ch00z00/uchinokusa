package postgres_test

import (
	"errors"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

func nov(day int) civil.Date { return civil.Date{Year: 2026, Month: 11, Day: day} }

// completeOn は、今の回を day の実施日で完了する。
func (f recordFixture) completeOn(day int) domain.CareRecord {
	f.t.Helper()
	task, err := f.repo.Get(f.ctx, f.home, f.task.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	rec, _, err := f.records.Complete(f.ctx, f.home, f.task.ID, f.sakura, domain.Completion{CycleSeq: task.CycleSeq, PerformedOn: nov(day)})
	if err != nil {
		f.t.Fatal(err)
	}
	return rec
}

func (f recordFixture) currentTask() domain.CareTask {
	f.t.Helper()
	task, err := f.repo.Get(f.ctx, f.home, f.task.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	return task
}

func TestCareRecordRepository_UpdatePerformedOn(t *testing.T) {
	t.Parallel()

	t.Run("最新の完了の実施日を変えると、変えた日から次回予定を計算し直す", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		rec := f.completeOn(4) // 次回予定は 11/11
		// 手で変えた予定日も、周期からの計算に戻す
		due := nov(20)
		if _, err := f.repo.Update(f.ctx, f.home, f.task.ID, domain.CareTaskPatch{NextDueOn: &due}); err != nil {
			t.Fatal(err)
		}

		got, task, err := f.records.UpdatePerformedOn(f.ctx, f.home, rec.ID, domain.RecordPatch{PerformedOn: nov(2)})
		if err != nil {
			t.Fatal(err)
		}
		if got.PerformedOn != nov(2) || task.NextDueOn != nov(9) || task.CycleSeq != 2 || task.DueOverridden {
			t.Errorf("record = %+v, task = %+v", got, task)
		}
		if n := f.count(`SELECT count(*) FROM care_records WHERE id = $1 AND edited_at IS NOT NULL`, rec.ID); n != 1 {
			t.Errorf("edited_at is not set")
		}
	})

	t.Run("古い完了の実施日を変えても、次回予定は変えない（手で変えた予定日も残す）", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		first := f.completeOn(1)
		f.completeOn(4)
		due := nov(20)
		if _, err := f.repo.Update(f.ctx, f.home, f.task.ID, domain.CareTaskPatch{NextDueOn: &due}); err != nil {
			t.Fatal(err)
		}
		if _, task, err := f.records.UpdatePerformedOn(f.ctx, f.home, first.ID, domain.RecordPatch{PerformedOn: nov(2)}); err != nil || task.NextDueOn != nov(20) || !task.DueOverridden {
			t.Errorf("task = %+v, %v; want next due 11/20 kept", task, err)
		}
	})

	t.Run("あとのスキップが決めた確認日は、完了の実施日を変えても残す", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		rec := f.completeOn(4)
		if _, _, err := f.records.Skip(f.ctx, f.home, f.task.ID, f.sakura, domain.Skip{CycleSeq: 2, PerformedOn: nov(4), RecheckOn: nov(12)}); err != nil {
			t.Fatal(err)
		}
		if _, task, err := f.records.UpdatePerformedOn(f.ctx, f.home, rec.ID, domain.RecordPatch{PerformedOn: nov(2)}); err != nil || task.NextDueOn != nov(12) {
			t.Errorf("task = %+v, %v; want next due 11/12", task, err)
		}
	})

	t.Run("削除したタスクの記録も編集でき、タスクは返さない", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		rec := f.completeOn(4)
		if err := f.repo.Delete(f.ctx, f.home, f.task.ID); err != nil {
			t.Fatal(err)
		}
		got, task, err := f.records.UpdatePerformedOn(f.ctx, f.home, rec.ID, domain.RecordPatch{PerformedOn: nov(2)})
		if err != nil || got.PerformedOn != nov(2) || task.ID != uuid.Nil {
			t.Errorf("record = %+v, task = %+v, %v", got, task, err)
		}
	})

	t.Run("ほかの世帯の記録は 404", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		rec := f.completeOn(4)
		if _, _, err := f.records.UpdatePerformedOn(f.ctx, f.other, rec.ID, domain.RecordPatch{PerformedOn: nov(2)}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestCareRecordRepository_Delete(t *testing.T) {
	t.Parallel()

	t.Run("最新の完了を消すと、残った最新の完了から次回予定を計算し直し、完了通知を送らない", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		f.completeOn(1)
		latest := f.completeOn(4)

		task, err := f.records.Delete(f.ctx, f.home, latest.ID)
		if err != nil {
			t.Fatal(err)
		}
		// 予定の回は戻さない（回は、完了するたびに進む番号）
		if task.NextDueOn != nov(8) || task.CycleSeq != 3 {
			t.Errorf("task = %+v, want next due 11/8, cycle 3", task)
		}
		if n := f.count(`SELECT count(*) FROM care_records WHERE id = $1`, latest.ID); n != 0 {
			t.Errorf("record is not deleted")
		}
		if n := f.count(`SELECT count(*) FROM notification_outbox WHERE status = 'canceled'`); n != 1 {
			t.Errorf("canceled outbox = %d, want 1", n)
		}
	})

	t.Run("完了が残らなければ、次回予定は変えない", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		rec := f.completeOn(4)
		if task, err := f.records.Delete(f.ctx, f.home, rec.ID); err != nil || task.NextDueOn != nov(11) {
			t.Errorf("task = %+v, %v; want next due 11/11", task, err)
		}
	})

	t.Run("ほかの世帯の記録は 404", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		rec := f.completeOn(4)
		if _, err := f.records.Delete(f.ctx, f.other, rec.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestCareRecordRepository_ListByPlant(t *testing.T) {
	t.Parallel()
	f := newRecordFixture(t)
	f.completeOn(1)
	f.completeOn(4)
	skip, _, err := f.records.Skip(f.ctx, f.home, f.task.ID, f.sakura, domain.Skip{CycleSeq: 3, Reason: domain.SkipSoilWet, PerformedOn: nov(5), RecheckOn: nov(6)})
	if err != nil {
		t.Fatal(err)
	}
	insert := func(plant uuid.UUID, kind, customName string, day int) {
		f.exec(`INSERT INTO care_records (household_id, plant_id, task_kind, task_custom_name, outcome, performed_on, actor_display_name, cycle_seq)
			VALUES ($1, $2, $3, NULLIF($4, ''), 'completed', $5, '退出したメンバー', 1)`, f.home, plant, kind, customName, nov(day))
	}
	insert(f.monsteraPlant, "fertilizing", "", 2)
	insert(f.monsteraPlant, "other", "葉水", 3)
	insert(f.pothosPlant, "watering", "", 4) // ほかの植物

	t.Run("新しい順に返し、cursor で次のページに進む", func(t *testing.T) {
		t.Parallel()
		page1, more, err := f.records.ListByPlant(f.ctx, f.home, f.monsteraPlant, domain.RecordQuery{Limit: 3})
		if err != nil || !more {
			t.Fatalf("more = %v, err = %v", more, err)
		}
		if got := dates(page1); len(got) != 3 || got[0] != nov(5) || got[1] != nov(4) || got[2] != nov(3) || page1[0].ID != skip.ID {
			t.Fatalf("page1 = %v", got)
		}
		after := domain.CursorOf(page1[2])
		page2, more, err := f.records.ListByPlant(f.ctx, f.home, f.monsteraPlant, domain.RecordQuery{Limit: 3, After: &after})
		if err != nil || more {
			t.Fatalf("more = %v, err = %v", more, err)
		}
		if got := dates(page2); len(got) != 2 || got[0] != nov(2) || got[1] != nov(1) {
			t.Errorf("page2 = %v", got)
		}
	})

	t.Run("種類で絞り込める", func(t *testing.T) {
		t.Parallel()
		got, _, err := f.records.ListByPlant(f.ctx, f.home, f.monsteraPlant, domain.RecordQuery{Kind: domain.CareWatering, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if d := dates(got); len(d) != 3 || got[0].SkipReason != domain.SkipSoilWet {
			t.Errorf("records = %v", d)
		}
	})

	t.Run("ほかの世帯からは見えない", func(t *testing.T) {
		t.Parallel()
		got, _, err := f.records.ListByPlant(f.ctx, f.other, f.monsteraPlant, domain.RecordQuery{Limit: 10})
		if err != nil || len(got) != 0 {
			t.Errorf("records = %v, %v", dates(got), err)
		}
	})

	t.Run("作業の種類（other は作業名）ごとの最終実施日。スキップは実施に数えない", func(t *testing.T) {
		t.Parallel()
		got, err := f.records.LastPerformed(f.ctx, f.home, f.monsteraPlant)
		if err != nil {
			t.Fatal(err)
		}
		want := []domain.LastPerformed{
			{Kind: domain.CareFertilizing, PerformedOn: nov(2)},
			{Kind: domain.CareOther, CustomName: "葉水", PerformedOn: nov(3)},
			{Kind: domain.CareWatering, PerformedOn: nov(4)},
		}
		if len(got) != len(want) {
			t.Fatalf("got %+v", got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %+v, want %+v", i, got[i], want[i])
			}
		}
	})
}

func dates(records []domain.CareRecord) []civil.Date {
	out := make([]civil.Date, len(records))
	for i, r := range records {
		out[i] = r.PerformedOn
	}
	return out
}
