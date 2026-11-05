package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeCareRecords は、app.CareRecordRepository の偽物である。受け取った完了を記録する。
type fakeCareRecords struct {
	completions []completionCall
	skips       []skipCall
	undos       []undoCall
	households  map[uuid.UUID]uuid.UUID // 記録の ID → 世帯
	fail        map[uuid.UUID]error     // タスクの ID → 完了で返すエラー
}

type undoCall struct {
	householdID, recordID, actorID uuid.UUID
	now                            time.Time
}

func (f *fakeCareRecords) HouseholdOf(_ context.Context, recordID uuid.UUID) (uuid.UUID, error) {
	if h, ok := f.households[recordID]; ok {
		return h, nil
	}
	return uuid.Nil, domain.ErrNotFound
}

func (f *fakeCareRecords) Undo(_ context.Context, householdID, recordID, actorID uuid.UUID, now time.Time) (domain.CareTask, error) {
	f.undos = append(f.undos, undoCall{householdID, recordID, actorID, now})
	return domain.CareTask{CycleSeq: 1}, nil
}

type skipCall struct {
	householdID, taskID, actorID uuid.UUID
	s                            domain.Skip
}

func (f *fakeCareRecords) Skip(_ context.Context, householdID, taskID, actorID uuid.UUID, s domain.Skip) (domain.CareRecord, domain.CareTask, error) {
	f.skips = append(f.skips, skipCall{householdID, taskID, actorID, s})
	return domain.CareRecord{TaskID: taskID, CycleSeq: s.CycleSeq, Outcome: domain.OutcomeSkipped}, domain.CareTask{ID: taskID}, nil
}

type completionCall struct {
	householdID, taskID, actorID uuid.UUID
	c                            domain.Completion
}

func (f *fakeCareRecords) Complete(_ context.Context, householdID, taskID, actorID uuid.UUID, c domain.Completion) (domain.CareRecord, domain.CareTask, error) {
	f.completions = append(f.completions, completionCall{householdID, taskID, actorID, c})
	if err := f.fail[taskID]; err != nil {
		return domain.CareRecord{}, domain.CareTask{}, err
	}
	return domain.CareRecord{TaskID: taskID, CycleSeq: c.CycleSeq, PerformedOn: c.PerformedOn}, domain.CareTask{ID: taskID}, nil
}

type recordFixture struct {
	taskFixture
	records *fakeCareRecords
	svc     *app.CareRecordService
	record  uuid.UUID // 世帯の記録
}

func newRecordFixture() recordFixture {
	tf := newTaskFixture()
	f := recordFixture{taskFixture: tf, record: uuid.Must(uuid.NewV7())}
	f.records = &fakeCareRecords{households: map[uuid.UUID]uuid.UUID{f.record: tf.household}, fail: map[uuid.UUID]error{}}
	f.svc = app.NewCareRecordService(f.records, tf.tasks, tf.authz, func() time.Time { return taskNow })
	return f
}

func TestCareRecordService_Complete(t *testing.T) {
	t.Parallel()

	t.Run("編集者は完了でき、実施日を省くと、世帯のタイムゾーンでの今日にする", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture()
		if _, _, err := f.svc.Complete(t.Context(), f.editor, f.watering, domain.CompletionInput{CycleSeq: ptrOf(1)}); err != nil {
			t.Fatal(err)
		}
		want := completionCall{f.household, f.watering, f.editor, domain.Completion{CycleSeq: 1, PerformedOn: civil.Date{Year: 2026, Month: 11, Day: 4}}}
		if len(f.records.completions) != 1 || f.records.completions[0] != want {
			t.Errorf("completions = %+v, want %+v", f.records.completions, want)
		}
	})

	t.Run("未来の実施日は 422 にあたるエラーで、記録しない", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture()
		// 東京では 11/4 なので、11/5 は未来
		_, _, err := f.svc.Complete(t.Context(), f.editor, f.watering, domain.CompletionInput{CycleSeq: ptrOf(1), PerformedOn: ptr("2026-11-05")})
		assertFields(t, err, map[string]string{"performedOn": "invalid"})
		if len(f.records.completions) != 0 {
			t.Errorf("completions = %+v, want none", f.records.completions)
		}
	})

	for _, tt := range taskWriteDenied {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newRecordFixture()
			// 入力が誤っていても、権限の確認を先に行う
			if _, _, err := f.svc.Complete(t.Context(), tt.user(f.taskFixture), tt.id(f.taskFixture), domain.CompletionInput{}); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(f.records.completions) != 0 {
				t.Errorf("completions = %+v, want none", f.records.completions)
			}
		})
	}
}

func TestCareRecordService_Skip(t *testing.T) {
	t.Parallel()

	t.Run("編集者はスキップでき、確認日を省くと、世帯のタイムゾーンでの翌日にする", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture()
		if _, _, err := f.svc.Skip(t.Context(), f.editor, f.watering, domain.SkipInput{CycleSeq: ptrOf(1), Reason: ptr("bad_weather")}); err != nil {
			t.Fatal(err)
		}
		want := skipCall{f.household, f.watering, f.editor, domain.Skip{
			CycleSeq: 1, Reason: domain.SkipBadWeather,
			PerformedOn: civil.Date{Year: 2026, Month: 11, Day: 4}, RecheckOn: civil.Date{Year: 2026, Month: 11, Day: 5},
		}}
		if len(f.records.skips) != 1 || f.records.skips[0] != want {
			t.Errorf("skips = %+v, want %+v", f.records.skips, want)
		}
	})

	t.Run("今日を確認日にすると 422 にあたるエラーで、記録しない", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture()
		// 東京では 11/4 が今日
		_, _, err := f.svc.Skip(t.Context(), f.editor, f.watering, domain.SkipInput{CycleSeq: ptrOf(1), RecheckOn: ptr("2026-11-04")})
		assertFields(t, err, map[string]string{"recheckOn": "invalid"})
		if len(f.records.skips) != 0 {
			t.Errorf("skips = %+v, want none", f.records.skips)
		}
	})

	for _, tt := range taskWriteDenied {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newRecordFixture()
			if _, _, err := f.svc.Skip(t.Context(), tt.user(f.taskFixture), tt.id(f.taskFixture), domain.SkipInput{}); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(f.records.skips) != 0 {
				t.Errorf("skips = %+v, want none", f.records.skips)
			}
		})
	}
}

func TestCareRecordService_Undo(t *testing.T) {
	t.Parallel()

	t.Run("編集者は、今の時刻とともに取り消しを頼む", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture()
		if _, err := f.svc.Undo(t.Context(), f.editor, f.record); err != nil {
			t.Fatal(err)
		}
		want := undoCall{f.household, f.record, f.editor, taskNow}
		if len(f.records.undos) != 1 || f.records.undos[0] != want {
			t.Errorf("undos = %+v, want %+v", f.records.undos, want)
		}
	})

	denied := []struct {
		name    string
		user    func(recordFixture) uuid.UUID
		record  func(recordFixture) uuid.UUID
		wantErr error
	}{
		{"閲覧者はできない", func(f recordFixture) uuid.UUID { return f.viewer }, func(f recordFixture) uuid.UUID { return f.record }, domain.ErrForbidden},
		{"ほかの世帯の記録は 404", func(f recordFixture) uuid.UUID { return f.stranger }, func(f recordFixture) uuid.UUID { return f.record }, domain.ErrNotFound},
		{"存在しない記録は 404", func(f recordFixture) uuid.UUID { return f.owner }, func(recordFixture) uuid.UUID { return uuid.Must(uuid.NewV7()) }, domain.ErrNotFound},
	}
	for _, tt := range denied {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newRecordFixture()
			if _, err := f.svc.Undo(t.Context(), tt.user(f), tt.record(f)); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(f.records.undos) != 0 {
				t.Errorf("undos = %+v, want none", f.records.undos)
			}
		})
	}
}

func TestCareRecordService_CompleteBatch(t *testing.T) {
	t.Parallel()

	t.Run("タスクごとに完了し、送った順に結果を返す。1件の失敗で、ほかを止めない", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture()
		dup := &domain.AlreadyCompletedError{}
		f.records.fail[f.other] = dup
		results, err := f.svc.CompleteBatch(t.Context(), f.editor, f.household, []app.BatchCompletionItem{
			{TaskID: f.other, Input: domain.CompletionInput{CycleSeq: ptrOf(1)}},
			{TaskID: f.watering, Input: domain.CompletionInput{}}, // 入力の誤りは、その項目だけのエラー
			{TaskID: f.watering, Input: domain.CompletionInput{CycleSeq: ptrOf(2)}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 3 {
			t.Fatalf("results = %+v", results)
		}
		if results[0].TaskID != f.other || !errors.Is(results[0].Err, dup) {
			t.Errorf("results[0] = %+v", results[0])
		}
		assertFields(t, results[1].Err, map[string]string{"cycleSeq": "required"})
		if results[2].Err != nil || results[2].Record.CycleSeq != 2 || results[2].Task.ID != f.watering {
			t.Errorf("results[2] = %+v", results[2])
		}
		// 実施日を省くと、世帯のタイムゾーンでの今日にする
		if len(f.records.completions) != 2 || f.records.completions[1].householdID != f.household ||
			f.records.completions[1].c.PerformedOn != (civil.Date{Year: 2026, Month: 11, Day: 4}) {
			t.Errorf("completions = %+v", f.records.completions)
		}
	})

	t.Run("件数は1〜50件", func(t *testing.T) {
		t.Parallel()
		f := newRecordFixture()
		_, err := f.svc.CompleteBatch(t.Context(), f.editor, f.household, nil)
		assertFields(t, err, map[string]string{"items": "required"})
		_, err = f.svc.CompleteBatch(t.Context(), f.editor, f.household, make([]app.BatchCompletionItem, app.MaxBatchCompletions+1))
		assertFields(t, err, map[string]string{"items": "too_many"})
		if len(f.records.completions) != 0 {
			t.Errorf("completions = %+v, want none", f.records.completions)
		}
	})

	for _, tt := range []struct {
		name    string
		user    func(recordFixture) uuid.UUID
		wantErr error
	}{
		{"閲覧者はできない", func(f recordFixture) uuid.UUID { return f.viewer }, domain.ErrForbidden},
		{"ほかの世帯は 404", func(f recordFixture) uuid.UUID { return f.stranger }, domain.ErrNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newRecordFixture()
			// 件数が誤っていても、権限の確認を先に行う
			if _, err := f.svc.CompleteBatch(t.Context(), tt.user(f), f.household, nil); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func ptrOf[T any](v T) *T { return &v }
