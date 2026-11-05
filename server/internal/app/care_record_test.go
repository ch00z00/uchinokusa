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

func ptrOf[T any](v T) *T { return &v }
