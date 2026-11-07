package job_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/advisor"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/job"
)

type stubAdvisor struct {
	id       string
	triggers []advisor.Trigger
}

func (a stubAdvisor) ID() string                  { return a.id }
func (a stubAdvisor) Triggers() []advisor.Trigger { return a.triggers }
func (a stubAdvisor) Advise(context.Context, advisor.Context) ([]advisor.Proposal, error) {
	return nil, nil
}

type fakePlantLister struct {
	ids   []uuid.UUID
	err   error
	calls int
}

func (f *fakePlantLister) ActiveIDs(context.Context) ([]uuid.UUID, error) {
	f.calls++
	return f.ids, f.err
}

type fakeEvaluator struct {
	events []app.AdviceEvent
}

func (f *fakeEvaluator) Evaluate(_ context.Context, ev app.AdviceEvent) []domain.Suggestion {
	f.events = append(f.events, ev)
	return nil
}

func registry(t *testing.T, advisors ...advisor.Advisor) *advisor.Registry {
	t.Helper()
	r, err := advisor.NewRegistry(advisors...)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDailyAdvice_Run(t *testing.T) {
	t.Parallel()
	plants := []uuid.UUID{uuid.New(), uuid.New()}

	t.Run("毎日のきっかけのプロバイダがあれば、植物ごとに評価する", func(t *testing.T) {
		t.Parallel()
		lister, eval := &fakePlantLister{ids: plants}, &fakeEvaluator{}
		r := registry(t, stubAdvisor{id: "weather.v1", triggers: []advisor.Trigger{advisor.TriggerDaily}})
		res, err := job.NewDailyAdvice(r, lister, eval).Run(t.Context())
		if err != nil || res != (job.Result{Processed: 2}) {
			t.Errorf("result = %+v, err = %v", res, err)
		}
		want := []app.AdviceEvent{{Trigger: advisor.TriggerDaily, PlantID: plants[0]}, {Trigger: advisor.TriggerDaily, PlantID: plants[1]}}
		if !slices.Equal(eval.events, want) {
			t.Errorf("events = %+v", eval.events)
		}
	})

	t.Run("対象のプロバイダがなければ（MVP）、植物も読まずに終わる", func(t *testing.T) {
		t.Parallel()
		lister, eval := &fakePlantLister{ids: plants}, &fakeEvaluator{}
		// 植え替えのプロバイダは、毎日のきっかけでは呼ばない
		r := registry(t, advisor.Repotting{})
		res, err := job.NewDailyAdvice(r, lister, eval).Run(t.Context())
		if err != nil || res != (job.Result{}) || lister.calls != 0 || len(eval.events) != 0 {
			t.Errorf("result = %+v, err = %v, calls = %d, events = %d", res, err, lister.calls, len(eval.events))
		}
	})

	t.Run("植物を読めなければエラーを返す", func(t *testing.T) {
		t.Parallel()
		r := registry(t, stubAdvisor{id: "weather.v1", triggers: []advisor.Trigger{advisor.TriggerDaily}})
		if _, err := job.NewDailyAdvice(r, &fakePlantLister{err: errors.New("db down")}, &fakeEvaluator{}).Run(t.Context()); err == nil {
			t.Error("err = nil")
		}
	})
}
