package postgres_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

var dec1 = civil.Date{Year: 2026, Month: 12, Day: 1}

// seasonFixture は、自宅の水やり（週1回、冬は2週に1回）を 11/25 に完了し、次回予定が 12/2 の状態である。
type seasonFixture struct {
	recordFixture
	seasons *postgres.SeasonRepository
	done    domain.CareRecord
}

func newSeasonFixture(t *testing.T) seasonFixture {
	t.Helper()
	f := seasonFixture{recordFixture: newRecordFixture(t)}
	f.seasons = postgres.NewSeasonRepository(f.pool)
	f.done = f.completeOn(f.task, civil.Date{Year: 2026, Month: 11, Day: 25})
	return f
}

func (f seasonFixture) completeOn(task domain.CareTask, on civil.Date) domain.CareRecord {
	f.t.Helper()
	current, err := f.repo.Get(f.ctx, f.home, task.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	rec, _, err := f.records.Complete(f.ctx, f.home, task.ID, f.sakura, domain.Completion{CycleSeq: current.CycleSeq, PerformedOn: on})
	if err != nil {
		f.t.Fatal(err)
	}
	return rec
}

func (f seasonFixture) nextDue(id uuid.UUID) civil.Date {
	f.t.Helper()
	task, err := f.repo.Get(f.ctx, f.home, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return task.NextDueOn
}

func TestSeasonRepository_Households(t *testing.T) {
	t.Parallel()
	f := newSeasonFixture(t)
	f.exec(`UPDATE households SET time_zone = 'Australia/Sydney', hemisphere = 'south' WHERE id = $1`, f.other)
	got, err := f.seasons.Households(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[uuid.UUID]domain.Household{
		f.home:  {ID: f.home, TimeZone: "Asia/Tokyo", Hemisphere: domain.HemisphereNorth},
		f.other: {ID: f.other, TimeZone: "Australia/Sydney", Hemisphere: domain.HemisphereSouth},
	}
	if len(got) != len(want) {
		t.Fatalf("households = %+v", got)
	}
	for _, h := range got {
		if h != want[h.ID] {
			t.Errorf("household = %+v, want %+v", h, want[h.ID])
		}
	}
}

func TestSeasonRepository_Recalculate(t *testing.T) {
	t.Parallel()

	t.Run("最後の完了に新しい季節の周期を足し、変えたタスクだけ変更イベントを出す", func(t *testing.T) {
		t.Parallel()
		f := newSeasonFixture(t)
		task := func(d domain.CareTaskDraft, kind domain.CareKind) domain.CareTask {
			d.Kind = kind
			return f.createTask(f.pothosPlant, d)
		}
		// 季節ごとの周期がない（週1回だけ）。予定日（11/8）を過ぎているが、季節の初日に動かさない
		plain := wateringDraft
		plain.Schedule.Seasonal = nil
		noSeason := task(plain, domain.CareFertilizing)
		f.completeOn(noSeason, civil.Date{Year: 2026, Month: 11, Day: 1})
		// 削除した
		deleted := f.createTask(f.monsteraPlant, func() domain.CareTaskDraft { d := wateringDraft; d.Kind = domain.CarePruning; return d }())
		f.completeOn(deleted, civil.Date{Year: 2026, Month: 11, Day: 25})
		if err := f.repo.Delete(f.ctx, f.home, deleted.ID); err != nil {
			t.Fatal(err)
		}
		// 手で予定日を変えた
		overridden := task(wateringDraft, domain.CarePruning)
		f.completeOn(overridden, civil.Date{Year: 2026, Month: 11, Day: 25})
		due := civil.Date{Year: 2026, Month: 12, Day: 3}
		if _, err := f.repo.Update(f.ctx, f.home, overridden.ID, domain.CareTaskPatch{NextDueOn: &due}); err != nil {
			t.Fatal(err)
		}
		// 一度も完了していない
		never := task(wateringDraft, domain.CareRepotting)
		// 一時停止中
		paused := f.createTask(f.monsteraPlant, func() domain.CareTaskDraft { d := wateringDraft; d.Kind = domain.CareFertilizing; return d }())
		f.completeOn(paused, civil.Date{Year: 2026, Month: 11, Day: 25})
		if _, err := f.repo.SetStatus(f.ctx, f.home, paused.ID, domain.TaskPaused, nil); err != nil {
			t.Fatal(err)
		}
		// 別の世帯の同じ形のタスク
		otherPlant, err := f.plantFixture.repo.Create(f.ctx, f.other, domain.PlantProfile{Name: "ポトス"})
		if err != nil {
			t.Fatal(err)
		}
		other, err := f.repo.Create(f.ctx, f.other, otherPlant.ID, wateringDraft)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := f.records.Complete(f.ctx, f.other, other.ID, f.sakura, domain.Completion{CycleSeq: 1, PerformedOn: civil.Date{Year: 2026, Month: 11, Day: 25}}); err != nil {
			t.Fatal(err)
		}
		before := map[uuid.UUID]civil.Date{}
		for _, id := range []uuid.UUID{noSeason.ID, overridden.ID, never.ID, paused.ID} {
			before[id] = f.nextDue(id)
		}

		w := watchChanges(t, f.pool)
		assertChanges(t, w, func() []change {
			n, err := f.seasons.Recalculate(f.ctx, f.home, dec1, domain.HemisphereNorth)
			if err != nil || n != 1 {
				t.Errorf("changed = %d, err = %v", n, err)
			}
			return []change{{f.home, domain.ChangeTask, f.task.ID, domain.ChangeUpdated, f.monsteraPlant}}
		})
		// 11/25 + 2週（冬の周期）
		if got := f.nextDue(f.task.ID); got != (civil.Date{Year: 2026, Month: 12, Day: 9}) {
			t.Errorf("next due = %v", got)
		}
		for id, want := range before {
			if got := f.nextDue(id); got != want {
				t.Errorf("task %s: next due = %v, want %v (unchanged)", id, got, want)
			}
		}
		var deletedDue string
		f.queryRow(`SELECT next_due_on::text FROM care_tasks WHERE id = $1`, deleted.ID).scan(&deletedDue)
		if deletedDue != "2026-12-02" {
			t.Errorf("deleted task: next due = %s", deletedDue)
		}
		if got, err := f.repo.Get(f.ctx, f.other, other.ID); err != nil || got.NextDueOn != (civil.Date{Year: 2026, Month: 12, Day: 2}) {
			t.Errorf("other household: %v, %v", got.NextDueOn, err)
		}

		// 初日のうちにもう一度実行しても、何も変えない
		assertChanges(t, w, func() []change {
			if n, err := f.seasons.Recalculate(f.ctx, f.home, dec1, domain.HemisphereNorth); err != nil || n != 0 {
				t.Errorf("again: changed = %d, err = %v", n, err)
			}
			return []change{}
		})
	})

	t.Run("追加の記録は起点にしない", func(t *testing.T) {
		t.Parallel()
		f := newSeasonFixture(t)
		// 完了済みの回（1回目）に、重複を承知で追加する
		if _, _, err := f.records.Complete(f.ctx, f.home, f.task.ID, f.sakura, domain.Completion{CycleSeq: f.done.CycleSeq, PerformedOn: civil.Date{Year: 2026, Month: 11, Day: 28}, AllowExtra: true}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.seasons.Recalculate(f.ctx, f.home, dec1, domain.HemisphereNorth); err != nil {
			t.Fatal(err)
		}
		if got := f.nextDue(f.task.ID); got != (civil.Date{Year: 2026, Month: 12, Day: 9}) {
			t.Errorf("next due = %v", got)
		}
	})

	t.Run("完了と重なったら、完了が確定してから、その完了を起点に計算し直す", func(t *testing.T) {
		t.Parallel()
		f := newSeasonFixture(t)
		// 完了（または記録の編集）のトランザクションが、タスクの行をロックしている
		tx, err := f.pool.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.WithoutCancel(f.ctx)) }()
		if _, err := tx.Exec(f.ctx, `SELECT 1 FROM care_tasks WHERE id = $1 FOR UPDATE`, f.task.ID); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			_, err := f.seasons.Recalculate(f.ctx, f.home, dec1, domain.HemisphereNorth)
			done <- err
		}()
		select {
		case err := <-done:
			t.Fatalf("recalculated while the task was locked: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
		// 実施日を 11/28 に直して確定する
		if _, err := tx.Exec(f.ctx, `UPDATE care_records SET performed_on = '2026-11-28' WHERE id = $1`, f.done.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(f.ctx, `UPDATE care_tasks SET next_due_on = '2026-12-05' WHERE id = $1`, f.task.ID); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		// 11/28 + 2週
		if got := f.nextDue(f.task.ID); got != (civil.Date{Year: 2026, Month: 12, Day: 12}) {
			t.Errorf("next due = %v", got)
		}
	})
}
