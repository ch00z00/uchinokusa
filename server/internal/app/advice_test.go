package app_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/advisor"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeAdvice は、app.AdviceEvaluator の偽物である。呼ばれたきっかけを記録する。
type fakeAdvice struct {
	events []app.AdviceEvent
}

func (f *fakeAdvice) Evaluate(_ context.Context, ev app.AdviceEvent) []domain.Suggestion {
	f.events = append(f.events, ev)
	return nil
}

func TestPlantService_Create_EvaluatesAdvice(t *testing.T) {
	t.Parallel()

	t.Run("登録した植物で、plant_created の提案を評価する", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		p, err := f.svc.Create(t.Context(), f.editor, f.household, domain.PlantInput{Name: ptr("パキラ"), AcquiredOn: ptr("2026-11-01")})
		if err != nil {
			t.Fatal(err)
		}
		if want := []app.AdviceEvent{{Trigger: advisor.TriggerPlantCreated, PlantID: p.ID}}; !reflect.DeepEqual(f.advice.events, want) {
			t.Errorf("events = %+v, want %+v", f.advice.events, want)
		}
	})

	t.Run("登録できなければ、評価しない", func(t *testing.T) {
		t.Parallel()
		f := newPlantFixture()
		if _, err := f.svc.Create(t.Context(), f.editor, f.household, domain.PlantInput{}); err == nil {
			t.Fatal("err = nil")
		}
		if len(f.advice.events) != 0 {
			t.Errorf("events = %+v", f.advice.events)
		}
	})
}

func TestCareTaskService_EvaluatesAdvice(t *testing.T) {
	t.Parallel()
	dueChanged := func(f taskFixture, task uuid.UUID) []app.AdviceEvent {
		return []app.AdviceEvent{{Trigger: advisor.TriggerTaskDueChanged, PlantID: f.plant, TaskID: task}}
	}

	t.Run("作ったタスクで評価する", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		task, err := f.svc.Create(t.Context(), f.editor, f.plant, domain.CareTaskInput{Kind: ptr("repotting"), Interval: &domain.IntervalInput{Value: 24, Unit: "month"}, NextDueOn: ptr("2027-07-20")})
		if err != nil {
			t.Fatal(err)
		}
		if want := dueChanged(f, task.ID); !reflect.DeepEqual(f.advice.events, want) {
			t.Errorf("events = %+v, want %+v", f.advice.events, want)
		}
	})

	t.Run("予定日を変えたら評価する", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		if _, err := f.svc.Update(t.Context(), f.editor, f.watering, domain.CareTaskInput{NextDueOn: ptr("2027-07-20")}); err != nil {
			t.Fatal(err)
		}
		if want := dueChanged(f, f.watering); !reflect.DeepEqual(f.advice.events, want) {
			t.Errorf("events = %+v, want %+v", f.advice.events, want)
		}
	})

	t.Run("予定日を変えなければ評価しない", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		if _, err := f.svc.Update(t.Context(), f.editor, f.watering, domain.CareTaskInput{ConditionText: ptr("乾いたら")}); err != nil {
			t.Fatal(err)
		}
		if len(f.advice.events) != 0 {
			t.Errorf("events = %+v", f.advice.events)
		}
	})

	// 一時停止・削除したタスクにはプロバイダが何も返さないので、前の警告が撤回される
	for _, tt := range []struct {
		name string
		op   func(taskFixture) error
	}{
		{"一時停止したら評価する", func(f taskFixture) error { _, err := f.svc.Pause(t.Context(), f.editor, f.watering); return err }},
		{"削除したら評価する", func(f taskFixture) error { return f.svc.Delete(t.Context(), f.editor, f.watering) }},
		{"再開したら評価する", func(f taskFixture) error {
			if _, err := f.svc.Pause(t.Context(), f.editor, f.watering); err != nil {
				return err
			}
			f.advice.events = nil
			_, err := f.svc.Resume(t.Context(), f.editor, f.watering)
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newTaskFixture()
			if err := tt.op(f); err != nil {
				t.Fatal(err)
			}
			if want := dueChanged(f, f.watering); !reflect.DeepEqual(f.advice.events, want) {
				t.Errorf("events = %+v, want %+v", f.advice.events, want)
			}
		})
	}
}
