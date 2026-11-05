package postgres_test

import (
	"errors"
	"sync"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// recordFixture は、記録のリポジトリの結合テストの共通の状態である。
// taskFixture に、home の水やりタスク（予定の回は 1）と、作業する人（さくら）を加える。
type recordFixture struct {
	taskFixture
	records *postgres.CareRecordRepository
	task    domain.CareTask
	sakura  uuid.UUID
}

func newRecordFixture(t *testing.T) recordFixture {
	t.Helper()
	f := recordFixture{taskFixture: newTaskFixture(t)}
	f.records = postgres.NewCareRecordRepository(f.pool)
	f.task = f.createTask(f.monsteraPlant, wateringDraft)
	f.sakura = uuid.MustParse(f.id(`INSERT INTO users (firebase_uid, display_name, auth_provider) VALUES ('sakura', 'さくら', 'google.com')`))
	return f
}

// performedOn は、wateringDraft（週1回、冬は2週に1回）の秋の実施日である。次回予定は1週間後になる。
var performedOn = civil.Date{Year: 2026, Month: 11, Day: 4}

func TestCareRecordRepository_Complete(t *testing.T) {
	t.Parallel()

	t.Run("今の回の完了は、記録を作り、次回予定と予定の回を進め、完了通知を予約する", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		// 手で変えた予定日も、完了すれば周期から計算し直す
		due := civil.Date{Year: 2026, Month: 11, Day: 6}
		if _, err := f.repo.Update(f.ctx, f.home, f.task.ID, domain.CareTaskPatch{NextDueOn: &due}); err != nil {
			t.Fatal(err)
		}

		rec, task, err := f.records.Complete(f.ctx, f.home, f.task.ID, f.sakura, domain.Completion{CycleSeq: 1, PerformedOn: performedOn})
		if err != nil {
			t.Fatal(err)
		}
		if rec.TaskID != f.task.ID || rec.PlantID != f.monsteraPlant || rec.TaskKind != domain.CareWatering || rec.Outcome != domain.OutcomeCompleted ||
			rec.PerformedOn != performedOn || rec.ActorUserID != f.sakura || rec.ActorDisplayName != "さくら" || rec.CycleSeq != 1 || rec.IsExtra {
			t.Errorf("record = %+v", rec)
		}
		if task.NextDueOn != (civil.Date{Year: 2026, Month: 11, Day: 11}) || task.CycleSeq != 2 || task.DueOverridden {
			t.Errorf("task = %+v", task)
		}
		// 取り消し（8.3）で戻す先として、記録前の予定日と回を残す
		var prevDue *string
		var prevSeq *int
		f.queryRow(`SELECT prev_next_due_on::text, prev_cycle_seq FROM care_records WHERE id = $1`, rec.ID).scan(&prevDue, &prevSeq)
		if prevDue == nil || *prevDue != "2026-11-06" || prevSeq == nil || *prevSeq != 1 {
			t.Errorf("prev = %v, %v", prevDue, prevSeq)
		}
		if n := f.count(`SELECT count(*) FROM notification_outbox WHERE kind = 'completion' AND record_id = $1 AND actor_user_id = $2 AND status = 'pending'`, rec.ID, f.sakura); n != 1 {
			t.Errorf("outbox = %d, want 1", n)
		}
	})

	t.Run("完了済みの回は記録せず、先に完了した記録と最新のタスクを返す", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		first, _, err := f.records.Complete(f.ctx, f.home, f.task.ID, f.sakura, domain.Completion{CycleSeq: 1, PerformedOn: performedOn})
		if err != nil {
			t.Fatal(err)
		}

		_, _, err = f.records.Complete(f.ctx, f.home, f.task.ID, f.sakura, domain.Completion{CycleSeq: 1, PerformedOn: performedOn})
		var dup *domain.AlreadyCompletedError
		if !errors.As(err, &dup) {
			t.Fatalf("err = %v, want AlreadyCompletedError", err)
		}
		if dup.Existing.ID != first.ID || dup.Existing.ActorDisplayName != "さくら" || dup.Task.CycleSeq != 2 {
			t.Errorf("dup = %+v", dup)
		}
		if n := f.count(`SELECT count(*) FROM care_records WHERE task_id = $1`, f.task.ID); n != 1 {
			t.Errorf("records = %d, want 1", n)
		}
	})

	t.Run("追加の記録の指定があれば、完了済みの回にも記録し、タスクの予定は変えない", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		_, advanced, err := f.records.Complete(f.ctx, f.home, f.task.ID, f.sakura, domain.Completion{CycleSeq: 1, PerformedOn: performedOn})
		if err != nil {
			t.Fatal(err)
		}

		rec, task, err := f.records.Complete(f.ctx, f.home, f.task.ID, f.sakura, domain.Completion{CycleSeq: 1, PerformedOn: performedOn, AllowExtra: true})
		if err != nil {
			t.Fatal(err)
		}
		if !rec.IsExtra || rec.CycleSeq != 1 || task.NextDueOn != advanced.NextDueOn || task.CycleSeq != advanced.CycleSeq {
			t.Errorf("record = %+v, task = %+v", rec, task)
		}
		var prevDue *string
		var prevSeq *int
		f.queryRow(`SELECT prev_next_due_on::text, prev_cycle_seq FROM care_records WHERE id = $1`, rec.ID).scan(&prevDue, &prevSeq)
		if prevDue != nil || prevSeq != nil {
			t.Errorf("prev = %v, %v, want NULL (undo does not change the task)", prevDue, prevSeq)
		}
	})

	t.Run("今の回と合わない回は stale_cycle", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		assertConflict(t, f.complete(5), domain.ConflictStaleCycle)
		// 今の回より前でも、完了の記録がなければ（取り消しなどで）完了済みとは言えない
		f.exec(`UPDATE care_tasks SET cycle_seq = 3 WHERE id = $1`, f.task.ID)
		assertConflict(t, f.complete(2), domain.ConflictStaleCycle)
		if n := f.count(`SELECT count(*) FROM care_records WHERE task_id = $1`, f.task.ID); n != 0 {
			t.Errorf("records = %d, want 0", n)
		}
	})

	t.Run("ほかの世帯のタスク・削除したタスクは 404", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		if _, _, err := f.records.Complete(f.ctx, f.other, f.task.ID, f.sakura, domain.Completion{CycleSeq: 1, PerformedOn: performedOn}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("other household: err = %v, want ErrNotFound", err)
		}
		if err := f.repo.Delete(f.ctx, f.home, f.task.ID); err != nil {
			t.Fatal(err)
		}
		if err := f.complete(1); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("deleted: err = %v, want ErrNotFound", err)
		}
	})

	t.Run("同じ回への同時の完了は、1件だけが成功し、残りは先の記録つきの重複になる", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		const n = 10
		errs := make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = f.complete(1)
			}()
		}
		wg.Wait()

		succeeded := 0
		for _, err := range errs {
			var dup *domain.AlreadyCompletedError
			switch {
			case err == nil:
				succeeded++
			case errors.As(err, &dup):
				// 行ロックで先のコミットを待つので、先の記録を返せる（ユニーク制約の違反にはならない）
				if dup.Existing.ID == uuid.Nil || dup.Task.CycleSeq != 2 {
					t.Errorf("dup = %+v", dup)
				}
			default:
				t.Errorf("err = %v", err)
			}
		}
		if succeeded != 1 {
			t.Errorf("succeeded = %d, want 1", succeeded)
		}
		if got := f.count(`SELECT count(*) FROM care_records WHERE task_id = $1`, f.task.ID); got != 1 {
			t.Errorf("records = %d, want 1", got)
		}
		if got := f.count(`SELECT count(*) FROM notification_outbox WHERE kind = 'completion'`); got != 1 {
			t.Errorf("outbox = %d, want 1", got)
		}
	})
}

func (f recordFixture) complete(cycleSeq int) error {
	_, _, err := f.records.Complete(f.ctx, f.home, f.task.ID, f.sakura, domain.Completion{CycleSeq: cycleSeq, PerformedOn: performedOn})
	return err
}

func assertConflict(t *testing.T, err error, code string) {
	t.Helper()
	var c *domain.ConflictError
	if !errors.As(err, &c) || c.Code != code {
		t.Errorf("err = %v, want conflict %q", err, code)
	}
}

func TestCareRecordRepository_Skip(t *testing.T) {
	t.Parallel()
	recheckOn := civil.Date{Year: 2026, Month: 11, Day: 5}
	skip := domain.Skip{CycleSeq: 1, Reason: domain.SkipOther, ReasonText: "旅行中", PerformedOn: performedOn, RecheckOn: recheckOn}

	t.Run("スキップは理由とともに記録し、次回予定を確認日にして、予定の回は進めない", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		before := f.task.NextDueOn

		rec, task, err := f.records.Skip(f.ctx, f.home, f.task.ID, f.sakura, skip)
		if err != nil {
			t.Fatal(err)
		}
		if rec.Outcome != domain.OutcomeSkipped || rec.TaskID != f.task.ID || rec.TaskKind != domain.CareWatering || rec.PerformedOn != performedOn ||
			rec.ActorDisplayName != "さくら" || rec.SkipReason != domain.SkipOther || rec.SkipReasonText != "旅行中" || rec.RecheckOn != recheckOn ||
			rec.CycleSeq != 1 || rec.IsExtra {
			t.Errorf("record = %+v", rec)
		}
		// 確認日はユーザーが決めた日なので、季節の切り替えで動かさない
		if task.NextDueOn != recheckOn || task.CycleSeq != 1 || !task.DueOverridden {
			t.Errorf("task = %+v", task)
		}
		var prevDue *string
		var prevSeq *int
		f.queryRow(`SELECT prev_next_due_on::text, prev_cycle_seq FROM care_records WHERE id = $1`, rec.ID).scan(&prevDue, &prevSeq)
		if prevDue == nil || *prevDue != before.String() || prevSeq == nil || *prevSeq != 1 {
			t.Errorf("prev = %v, %v, want %s, 1", prevDue, prevSeq, before)
		}
		// スキップは完了通知を送らない（要件16-7）
		if n := f.count(`SELECT count(*) FROM notification_outbox`); n != 0 {
			t.Errorf("outbox = %d, want 0", n)
		}
	})

	t.Run("理由なしのスキップは、理由を NULL にする", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		rec, _, err := f.records.Skip(f.ctx, f.home, f.task.ID, f.sakura, domain.Skip{CycleSeq: 1, PerformedOn: performedOn, RecheckOn: recheckOn})
		if err != nil {
			t.Fatal(err)
		}
		if n := f.count(`SELECT count(*) FROM care_records WHERE id = $1 AND skip_reason_code IS NULL AND skip_reason_text IS NULL`, rec.ID); n != 1 {
			t.Errorf("reason is not NULL")
		}
	})

	t.Run("スキップした回に、あとから完了できる", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		if _, _, err := f.records.Skip(f.ctx, f.home, f.task.ID, f.sakura, skip); err != nil {
			t.Fatal(err)
		}
		// 同じ回を続けてスキップすることもできる（翌日も湿っていた場合）
		if _, _, err := f.records.Skip(f.ctx, f.home, f.task.ID, f.sakura, skip); err != nil {
			t.Fatal(err)
		}
		_, task, err := f.records.Complete(f.ctx, f.home, f.task.ID, f.sakura, domain.Completion{CycleSeq: 1, PerformedOn: performedOn})
		if err != nil {
			t.Fatal(err)
		}
		if task.CycleSeq != 2 || task.NextDueOn != (civil.Date{Year: 2026, Month: 11, Day: 11}) || task.DueOverridden {
			t.Errorf("task = %+v", task)
		}
	})

	t.Run("今の回でなければ stale_cycle で、記録しない", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		if err := f.complete(1); err != nil {
			t.Fatal(err)
		}
		// 別のメンバーが先に完了したあとの、古い表示からのスキップ
		_, _, err := f.records.Skip(f.ctx, f.home, f.task.ID, f.sakura, skip)
		assertConflict(t, err, domain.ConflictStaleCycle)
		if n := f.count(`SELECT count(*) FROM care_records WHERE outcome = 'skipped'`); n != 0 {
			t.Errorf("skips = %d, want 0", n)
		}
	})

	t.Run("ほかの世帯のタスクは 404", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture(t)
		if _, _, err := f.records.Skip(f.ctx, f.other, f.task.ID, f.sakura, skip); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}
