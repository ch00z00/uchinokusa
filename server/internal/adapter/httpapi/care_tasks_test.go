package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
	"github.com/ch00z00/uchinokusa/server/internal/logging"
)

var testTask = domain.CareTask{
	ID:          uuid.MustParse("01920000-0000-7000-8000-0000000000e1"),
	HouseholdID: testHousehold.ID,
	PlantID:     testPlant.ID,
	CareTaskProfile: domain.CareTaskProfile{
		Kind: domain.CareWatering,
		Schedule: domain.Schedule{
			Base:     domain.Interval{Value: 1, Unit: domain.IntervalWeek},
			Seasonal: domain.SeasonalIntervals{domain.SeasonWinter: {Value: 2, Unit: domain.IntervalWeek}},
		},
		ConditionText: "土の表面が乾いていたら",
	},
	NextDueOn: civil.Date{Year: 2026, Month: 11, Day: 8},
	CycleSeq:  3,
	Status:    domain.TaskActive,
	CreatedAt: time.Date(2026, 11, 4, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
}

// fakeCareTaskService は、app.CareTaskService の偽物である。
// testTask は testPlant のタスクで、ほかの植物・タスクは見つからないものとして振る舞う。受け取った入力を記録する。
type fakeCareTaskService struct {
	inputs  []domain.CareTaskInput
	deleted []uuid.UUID
}

func (f *fakeCareTaskService) List(_ context.Context, _, plantID uuid.UUID) ([]domain.CareTask, error) {
	if plantID != testPlant.ID {
		return nil, domain.ErrNotFound
	}
	return []domain.CareTask{testTask}, nil
}

func (f *fakeCareTaskService) Defaults(_ context.Context, _, plantID uuid.UUID) ([]domain.TaskDefault, error) {
	if plantID != testPlant.ID {
		return nil, domain.ErrNotFound
	}
	return []domain.TaskDefault{{Kind: domain.CareWatering, ConditionText: "鉢土の表面が乾いてきたら"}}, nil
}

func (f *fakeCareTaskService) Create(_ context.Context, _, plantID uuid.UUID, in domain.CareTaskInput) (domain.CareTask, error) {
	f.inputs = append(f.inputs, in)
	if plantID != testPlant.ID {
		return domain.CareTask{}, domain.ErrNotFound
	}
	d, err := domain.NewCareTask(in, civil.Date{Year: 2026, Month: 11, Day: 4})
	if err != nil {
		return domain.CareTask{}, err
	}
	return domain.CareTask{ID: testTask.ID, HouseholdID: testHousehold.ID, PlantID: plantID, CareTaskProfile: d.CareTaskProfile, NextDueOn: d.NextDueOn, CycleSeq: 1, Status: domain.TaskActive, CreatedAt: testTask.CreatedAt}, nil
}

func (f *fakeCareTaskService) Update(_ context.Context, _, taskID uuid.UUID, in domain.CareTaskInput) (domain.CareTask, error) {
	f.inputs = append(f.inputs, in)
	if taskID != testTask.ID {
		return domain.CareTask{}, domain.ErrNotFound
	}
	return testTask, nil
}

func (f *fakeCareTaskService) Delete(_ context.Context, _, taskID uuid.UUID) error {
	if taskID != testTask.ID {
		return domain.ErrForbidden
	}
	f.deleted = append(f.deleted, taskID)
	return nil
}

func (f *fakeCareTaskService) Pause(_ context.Context, _, taskID uuid.UUID) (domain.CareTask, error) {
	if taskID != testTask.ID {
		return domain.CareTask{}, domain.ErrNotFound
	}
	t := testTask
	t.Status = domain.TaskPaused
	return t, nil
}

func (f *fakeCareTaskService) Resume(_ context.Context, _, taskID uuid.UUID) (domain.CareTask, error) {
	if taskID != testTask.ID {
		return domain.CareTask{}, domain.ErrForbidden
	}
	return testTask, nil
}

func newTaskRouter(t *testing.T, tasks *fakeCareTaskService) http.Handler {
	t.Helper()
	logger := slog.New(logging.NewHandler(&bytes.Buffer{}, slog.LevelInfo))
	return httpapi.NewRouter(httpapi.Deps{Logger: logger, Verifier: fakeVerifier{}, Accounts: &fakeAccounts{}, Tasks: tasks})
}

func TestListCareTasks(t *testing.T) {
	t.Parallel()

	t.Run("季節ごとの周期と予定の回の番号つきで返し、未設定の項目は省く", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newTaskRouter(t, &fakeCareTaskService{}), http.MethodGet, "/v1/plants/"+testPlant.ID.String()+"/tasks", "Bearer "+validToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		const want = `{"tasks":[{"conditionText":"土の表面が乾いていたら","createdAt":"2026-11-04T00:00:00Z","cycleSeq":3,` +
			`"id":"01920000-0000-7000-8000-0000000000e1","interval":{"unit":"week","value":1},"kind":"watering","nextDueOn":"2026-11-08",` +
			`"plantId":"01920000-0000-7000-8000-0000000000d1","seasonalIntervals":{"winter":{"unit":"week","value":2}},"status":"active"}]}` + "\n"
		if got := rec.Body.String(); got != want {
			t.Errorf("body =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("見つからない植物は 404", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newTaskRouter(t, &fakeCareTaskService{}), http.MethodGet, "/v1/plants/"+uuid.Must(uuid.NewV7()).String()+"/tasks", "Bearer "+validToken, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestListCareTaskDefaults(t *testing.T) {
	t.Parallel()
	rec := serve(t, newTaskRouter(t, &fakeCareTaskService{}), http.MethodGet, "/v1/plants/"+testPlant.ID.String()+"/task-defaults", "Bearer "+validToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	const want = `{"defaults":[{"conditionText":"鉢土の表面が乾いてきたら","kind":"watering"}]}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}
}

func TestCreateCareTask(t *testing.T) {
	t.Parallel()

	t.Run("作成したタスクを 201 で返し、季節ごとの周期は送った季節だけを渡す", func(t *testing.T) {
		t.Parallel()
		tasks := &fakeCareTaskService{}
		rec := serve(t, newTaskRouter(t, tasks), http.MethodPost, "/v1/plants/"+testPlant.ID.String()+"/tasks", "Bearer "+validToken,
			`{"kind": "other", "customName": "葉水", "interval": {"value": 3, "unit": "day"}, "seasonalIntervals": {"summer": {"value": 1, "unit": "day"}}}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		got := decode[struct {
			Kind, CustomName, NextDueOn string
			CycleSeq                    int
		}](t, rec)
		if got.Kind != "other" || got.CustomName != "葉水" || got.NextDueOn != "2026-11-04" || got.CycleSeq != 1 {
			t.Errorf("body = %+v", got)
		}
		in := tasks.inputs[0]
		if in.Kind == nil || *in.Kind != "other" || in.Interval == nil || *in.Interval != (domain.IntervalInput{Value: 3, Unit: "day"}) ||
			len(in.SeasonalIntervals) != 1 || in.SeasonalIntervals["summer"] != (domain.IntervalInput{Value: 1, Unit: "day"}) ||
			in.ConditionText != nil || in.Note != nil || in.NextDueOn != nil {
			t.Errorf("input = %+v", in)
		}
	})

	t.Run("季節ごとの周期を送らなければ nil を渡す", func(t *testing.T) {
		t.Parallel()
		tasks := &fakeCareTaskService{}
		rec := serve(t, newTaskRouter(t, tasks), http.MethodPost, "/v1/plants/"+testPlant.ID.String()+"/tasks", "Bearer "+validToken,
			`{"kind": "watering", "interval": {"value": 7, "unit": "day"}}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if tasks.inputs[0].SeasonalIntervals != nil {
			t.Errorf("seasonal = %+v, want nil", tasks.inputs[0].SeasonalIntervals)
		}
		if got := decode[struct{ SeasonalIntervals map[string]any }](t, rec); got.SeasonalIntervals == nil || len(got.SeasonalIntervals) != 0 {
			t.Errorf("seasonalIntervals = %v, want {}", got.SeasonalIntervals)
		}
	})

	t.Run("入力の誤りは 422", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newTaskRouter(t, &fakeCareTaskService{}), http.MethodPost, "/v1/plants/"+testPlant.ID.String()+"/tasks", "Bearer "+validToken,
			`{"kind": "misting", "interval": {"value": 0, "unit": "day"}, "seasonalIntervals": {"winter": {"value": 1, "unit": "year"}}}`)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rec.Code)
		}
		if got := decode[errorBody](t, rec); got.Fields["kind"] != "invalid" || got.Fields["interval"] != "invalid" || got.Fields["seasonalIntervals.winter"] != "invalid" {
			t.Errorf("fields = %v", got.Fields)
		}
	})
}

func TestUpdateCareTask(t *testing.T) {
	t.Parallel()

	t.Run("空の季節ごとの周期は、なくす指示として空の map を渡す", func(t *testing.T) {
		t.Parallel()
		tasks := &fakeCareTaskService{}
		rec := serve(t, newTaskRouter(t, tasks), http.MethodPatch, "/v1/tasks/"+testTask.ID.String(), "Bearer "+validToken,
			`{"seasonalIntervals": {}, "conditionText": "", "nextDueOn": "2026-11-20"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		in := tasks.inputs[0]
		if in.SeasonalIntervals == nil || len(in.SeasonalIntervals) != 0 || in.ConditionText == nil || *in.ConditionText != "" ||
			in.NextDueOn == nil || *in.NextDueOn != "2026-11-20" || in.Interval != nil || in.Note != nil || in.CustomName != nil || in.Kind != nil {
			t.Errorf("input = %+v", in)
		}
	})

	t.Run("見つからないタスクは 404", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newTaskRouter(t, &fakeCareTaskService{}), http.MethodPatch, "/v1/tasks/"+uuid.Must(uuid.NewV7()).String(), "Bearer "+validToken, `{"note": "x"}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestDeleteCareTask(t *testing.T) {
	t.Parallel()

	t.Run("削除したら 204", func(t *testing.T) {
		t.Parallel()
		tasks := &fakeCareTaskService{}
		rec := serve(t, newTaskRouter(t, tasks), http.MethodDelete, "/v1/tasks/"+testTask.ID.String(), "Bearer "+validToken, "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		if len(tasks.deleted) != 1 {
			t.Errorf("deleted = %v", tasks.deleted)
		}
	})

	t.Run("権限がなければ 403", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, newTaskRouter(t, &fakeCareTaskService{}), http.MethodDelete, "/v1/tasks/"+uuid.Must(uuid.NewV7()).String(), "Bearer "+validToken, "")
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})
}

func TestPauseAndResumeCareTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, path string
		id         uuid.UUID
		wantCode   int
		wantStatus string
	}{
		{"一時停止したタスクを 200 で返す", "/pause", testTask.ID, http.StatusOK, "paused"},
		{"再開したタスクを 200 で返す", "/resume", testTask.ID, http.StatusOK, "active"},
		{"一時停止で見つからないタスクは 404", "/pause", uuid.Must(uuid.NewV7()), http.StatusNotFound, ""},
		{"再開の権限がなければ 403", "/resume", uuid.Must(uuid.NewV7()), http.StatusForbidden, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := serve(t, newTaskRouter(t, &fakeCareTaskService{}), http.MethodPost, "/v1/tasks/"+tt.id.String()+tt.path, "Bearer "+validToken, "")
			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantStatus == "" {
				return
			}
			if got := decode[struct{ ID, Status string }](t, rec); got.ID != testTask.ID.String() || got.Status != tt.wantStatus {
				t.Errorf("body = %+v", got)
			}
		})
	}
}
