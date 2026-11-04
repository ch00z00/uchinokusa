package postgres_test

import (
	"errors"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// taskFixture は、タスクのリポジトリの結合テストの共通の状態である。
// 世帯（home）の植物（monstera：マスタの目安つき、pothos：マスタなし）と、ほかの世帯（other）を用意する。
type taskFixture struct {
	plantFixture
	repo                       *postgres.CareTaskRepository
	monsteraPlant, pothosPlant uuid.UUID
}

func newTaskFixture(t *testing.T) taskFixture {
	t.Helper()
	f := taskFixture{plantFixture: newPlantFixture(t)}
	f.repo = postgres.NewCareTaskRepository(f.pool)
	f.exec(`UPDATE species SET recommended_care = '{"watering": {"condition": "鉢土の表面が乾いてきたら"}}' WHERE id = $1`, f.plantFixture.monstera)
	f.monsteraPlant = f.create("モンステラ", domain.PlantProfile{Species: domain.PlantSpecies{ID: f.plantFixture.monstera}}).ID
	f.pothosPlant = f.create("ポトス", domain.PlantProfile{}).ID
	return f
}

var wateringDraft = domain.CareTaskDraft{
	CareTaskProfile: domain.CareTaskProfile{
		Kind: domain.CareWatering,
		Schedule: domain.Schedule{
			Base:     domain.Interval{Value: 1, Unit: domain.IntervalWeek},
			Seasonal: domain.SeasonalIntervals{domain.SeasonWinter: {Value: 2, Unit: domain.IntervalWeek}},
		},
		ConditionText: "土の表面が乾いていたら",
		Note:          "底から流れるまでたっぷり",
	},
	NextDueOn: civil.Date{Year: 2026, Month: 11, Day: 4},
}

func (f taskFixture) createTask(plantID uuid.UUID, d domain.CareTaskDraft) domain.CareTask {
	f.t.Helper()
	task, err := f.repo.Create(f.ctx, f.home, plantID, d)
	if err != nil {
		f.t.Fatal(err)
	}
	return task
}

func TestCareTaskRepository_TaskPlantOf(t *testing.T) {
	t.Parallel()
	f := newTaskFixture(t)

	got, err := f.repo.TaskPlantOf(f.ctx, f.monsteraPlant)
	if err != nil {
		t.Fatal(err)
	}
	if got.HouseholdID != f.home || got.TimeZone != "Asia/Tokyo" || got.Care == nil || got.Care.Watering == nil || got.Care.Watering.Condition != "鉢土の表面が乾いてきたら" {
		t.Errorf("monstera = %+v", got)
	}
	if got, err := f.repo.TaskPlantOf(f.ctx, f.pothosPlant); err != nil || got.Care != nil {
		t.Errorf("pothos (no species) = %+v, %v, want nil care", got, err)
	}
	if _, err := f.repo.TaskPlantOf(f.ctx, uuid.Must(uuid.NewV7())); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCareTaskRepository_CreateAndList(t *testing.T) {
	t.Parallel()
	f := newTaskFixture(t)

	created := f.createTask(f.monsteraPlant, wateringDraft)
	if created.ID == uuid.Nil || created.HouseholdID != f.home || created.PlantID != f.monsteraPlant || created.CycleSeq != 1 ||
		created.Status != domain.TaskActive || created.DueOverridden || created.NextDueOn != wateringDraft.NextDueOn || created.CreatedAt.IsZero() {
		t.Errorf("created = %+v", created)
	}
	if c := created.CareTaskProfile; c.Kind != domain.CareWatering || c.Schedule.Base != wateringDraft.Schedule.Base ||
		len(c.Schedule.Seasonal) != 1 || c.Schedule.Seasonal[domain.SeasonWinter] != wateringDraft.Schedule.Seasonal[domain.SeasonWinter] ||
		c.ConditionText != wateringDraft.ConditionText || c.Note != wateringDraft.Note || c.CustomName != "" {
		t.Errorf("profile = %+v", c)
	}

	other := f.createTask(f.monsteraPlant, domain.CareTaskDraft{
		CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareOther, CustomName: "葉水", Schedule: domain.Schedule{Base: domain.Interval{Value: 3, Unit: domain.IntervalDay}}},
		NextDueOn:       civil.Date{Year: 2026, Month: 11, Day: 5},
	})
	if other.CustomName != "葉水" || other.ConditionText != "" || len(other.Schedule.Seasonal) != 0 {
		t.Errorf("other = %+v", other)
	}
	paused := f.createTask(f.monsteraPlant, wateringDraft)
	f.exec(`UPDATE care_tasks SET status = 'paused' WHERE id = $1`, paused.ID)
	deleted := f.createTask(f.monsteraPlant, wateringDraft)
	if err := f.repo.Delete(f.ctx, f.home, deleted.ID); err != nil {
		t.Fatal(err)
	}
	f.createTask(f.pothosPlant, wateringDraft)

	got, err := f.repo.List(f.ctx, f.home, f.monsteraPlant)
	if err != nil {
		t.Fatal(err)
	}
	// 作成の古い順に、削除したタスクとほかの植物のタスクを除いて返す。一時停止中のタスクは含める
	if len(got) != 3 || got[0].ID != created.ID || got[1].ID != other.ID || got[2].ID != paused.ID || got[2].Status != domain.TaskPaused {
		t.Errorf("tasks = %+v", got)
	}
	if empty, err := f.repo.List(f.ctx, f.other, f.monsteraPlant); err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("other household: %v, %v (want [] not nil)", empty, err)
	}
}

func TestCareTaskRepository_Update(t *testing.T) {
	t.Parallel()

	t.Run("周期・季節ごとの周期・実行条件を変えても、予定日はそのまま", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture(t)
		task := f.createTask(f.monsteraPlant, wateringDraft)

		got, err := f.repo.Update(f.ctx, f.home, task.ID, domain.CareTaskPatch{
			Interval:      &domain.Interval{Value: 10, Unit: domain.IntervalDay},
			Seasonal:      &domain.SeasonalIntervals{},
			ConditionText: ptrOf(""),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Schedule.Base != (domain.Interval{Value: 10, Unit: domain.IntervalDay}) || len(got.Schedule.Seasonal) != 0 || got.ConditionText != "" ||
			got.Note != wateringDraft.Note || got.NextDueOn != task.NextDueOn || got.DueOverridden {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("予定日を手で変えると、手で変えた印を立てる", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture(t)
		task := f.createTask(f.monsteraPlant, wateringDraft)

		due := civil.Date{Year: 2026, Month: 11, Day: 20}
		got, err := f.repo.Update(f.ctx, f.home, task.ID, domain.CareTaskPatch{NextDueOn: &due})
		if err != nil {
			t.Fatal(err)
		}
		if got.NextDueOn != due || !got.DueOverridden || got.CycleSeq != task.CycleSeq {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("ほかの世帯のタスク・削除したタスクは変更できない", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture(t)
		task := f.createTask(f.monsteraPlant, wateringDraft)
		if _, err := f.repo.Update(f.ctx, f.other, task.ID, domain.CareTaskPatch{Note: ptrOf("x")}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("other household: err = %v, want ErrNotFound", err)
		}
		if got, _ := f.repo.Get(f.ctx, f.home, task.ID); got.Note != wateringDraft.Note {
			t.Errorf("note = %q, want unchanged", got.Note)
		}

		if err := f.repo.Delete(f.ctx, f.home, task.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repo.Update(f.ctx, f.home, task.ID, domain.CareTaskPatch{Note: ptrOf("x")}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("deleted: err = %v, want ErrNotFound", err)
		}
	})
}

func TestCareTaskRepository_Delete(t *testing.T) {
	t.Parallel()

	t.Run("論理削除して記録は残し、以後は見つからない", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture(t)
		task := f.createTask(f.monsteraPlant, wateringDraft)
		f.record(task.ID.String(), "completed", 1, false)

		if err := f.repo.Delete(f.ctx, f.home, task.ID); err != nil {
			t.Fatal(err)
		}
		if n := f.count(`SELECT count(*) FROM care_tasks WHERE id = $1 AND deleted_at IS NOT NULL`, task.ID); n != 1 {
			t.Errorf("soft-deleted rows = %d, want 1", n)
		}
		if n := f.count(`SELECT count(*) FROM care_records WHERE task_id = $1`, task.ID); n != 1 {
			t.Errorf("records = %d, want 1 (kept as history)", n)
		}
		if _, err := f.repo.HouseholdOf(f.ctx, task.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("HouseholdOf: err = %v, want ErrNotFound", err)
		}
		if err := f.repo.Delete(f.ctx, f.home, task.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("delete again: err = %v, want ErrNotFound", err)
		}
	})

	t.Run("ほかの世帯のタスクは削除できない", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture(t)
		task := f.createTask(f.monsteraPlant, wateringDraft)
		if err := f.repo.Delete(f.ctx, f.other, task.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		if got, err := f.repo.HouseholdOf(f.ctx, task.ID); err != nil || got != f.home {
			t.Errorf("HouseholdOf = %v, %v, want still present", got, err)
		}
	})
}

func TestCareTaskRepository_SetStatus(t *testing.T) {
	t.Parallel()

	t.Run("一時停止では次回予定を変えず、再開では渡した次回予定にする", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture(t)
		task := f.createTask(f.monsteraPlant, wateringDraft)

		paused, err := f.repo.SetStatus(f.ctx, f.home, task.ID, domain.TaskPaused, nil)
		if err != nil {
			t.Fatal(err)
		}
		if paused.Status != domain.TaskPaused || paused.NextDueOn != task.NextDueOn {
			t.Errorf("paused = %+v", paused)
		}

		due := civil.Date{Year: 2026, Month: 11, Day: 10}
		resumed, err := f.repo.SetStatus(f.ctx, f.home, task.ID, domain.TaskActive, &due)
		if err != nil {
			t.Fatal(err)
		}
		// 再開で決めた予定日は、手で変えた予定日ではないので、手で変えた印は立てない
		if resumed.Status != domain.TaskActive || resumed.NextDueOn != due || resumed.DueOverridden || resumed.CycleSeq != task.CycleSeq {
			t.Errorf("resumed = %+v", resumed)
		}
	})

	t.Run("ほかの世帯のタスク・削除したタスクは変えられない", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture(t)
		task := f.createTask(f.monsteraPlant, wateringDraft)
		if _, err := f.repo.SetStatus(f.ctx, f.other, task.ID, domain.TaskPaused, nil); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("other household: err = %v, want ErrNotFound", err)
		}
		if got, _ := f.repo.Get(f.ctx, f.home, task.ID); got.Status != domain.TaskActive {
			t.Errorf("status = %q, want unchanged", got.Status)
		}

		if err := f.repo.Delete(f.ctx, f.home, task.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repo.SetStatus(f.ctx, f.home, task.ID, domain.TaskPaused, nil); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("deleted: err = %v, want ErrNotFound", err)
		}
	})
}

func TestCareTaskRepository_TimeZoneOf(t *testing.T) {
	t.Parallel()
	f := newTaskFixture(t)
	f.exec(`UPDATE households SET time_zone = 'America/Los_Angeles' WHERE id = $1`, f.other)

	if got, err := f.repo.TimeZoneOf(f.ctx, f.home); err != nil || got != "Asia/Tokyo" {
		t.Errorf("home = %q, %v", got, err)
	}
	if got, err := f.repo.TimeZoneOf(f.ctx, f.other); err != nil || got != "America/Los_Angeles" {
		t.Errorf("other = %q, %v", got, err)
	}
	if _, err := f.repo.TimeZoneOf(f.ctx, uuid.Must(uuid.NewV7())); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown: err = %v, want ErrNotFound", err)
	}
}
