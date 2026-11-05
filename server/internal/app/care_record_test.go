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
	return domain.CareRecord{TaskID: taskID, CycleSeq: c.CycleSeq, PerformedOn: c.PerformedOn}, domain.CareTask{ID: taskID}, nil
}

type recordFixture struct {
	taskFixture
	records *fakeCareRecords
	svc     *app.CareRecordService
}

func newRecordFixture() recordFixture {
	tf := newTaskFixture()
	f := recordFixture{taskFixture: tf, records: &fakeCareRecords{}}
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

func ptrOf[T any](v T) *T { return &v }
