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

// fakeCareTasks は、app.CareTaskRepository の偽物である。植物とタスクをメモリに持ち、呼ばれた操作を記録する。
type fakeCareTasks struct {
	plants   map[uuid.UUID]app.TaskPlant
	byID     map[uuid.UUID]domain.CareTask
	created  []domain.CareTaskDraft
	patched  []domain.CareTaskPatch
	deleted  []uuid.UUID
	statuses []domain.TaskStatus // SetStatus で変えた状態
}

func (f *fakeCareTasks) TaskPlantOf(_ context.Context, plantID uuid.UUID) (app.TaskPlant, error) {
	p, ok := f.plants[plantID]
	if !ok {
		return app.TaskPlant{}, domain.ErrNotFound
	}
	return p, nil
}

func (f *fakeCareTasks) List(_ context.Context, householdID, plantID uuid.UUID) ([]domain.CareTask, error) {
	out := []domain.CareTask{}
	for _, t := range f.byID {
		if t.HouseholdID == householdID && t.PlantID == plantID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeCareTasks) Create(_ context.Context, householdID, plantID uuid.UUID, d domain.CareTaskDraft) (domain.CareTask, error) {
	f.created = append(f.created, d)
	t := domain.CareTask{ID: uuid.Must(uuid.NewV7()), HouseholdID: householdID, PlantID: plantID, CareTaskProfile: d.CareTaskProfile, NextDueOn: d.NextDueOn, CycleSeq: 1, Status: domain.TaskActive}
	f.byID[t.ID] = t
	return t, nil
}

func (f *fakeCareTasks) HouseholdOf(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	t, ok := f.byID[id]
	if !ok {
		return uuid.Nil, domain.ErrNotFound
	}
	return t.HouseholdID, nil
}

func (f *fakeCareTasks) Get(_ context.Context, householdID, id uuid.UUID) (domain.CareTask, error) {
	t, ok := f.byID[id]
	if !ok || t.HouseholdID != householdID {
		return domain.CareTask{}, domain.ErrNotFound
	}
	return t, nil
}

func (f *fakeCareTasks) Update(ctx context.Context, householdID, id uuid.UUID, p domain.CareTaskPatch) (domain.CareTask, error) {
	f.patched = append(f.patched, p)
	return f.Get(ctx, householdID, id)
}

func (f *fakeCareTasks) Delete(ctx context.Context, householdID, id uuid.UUID) error {
	if _, err := f.Get(ctx, householdID, id); err != nil {
		return err
	}
	delete(f.byID, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeCareTasks) TimeZoneOf(_ context.Context, householdID uuid.UUID) (string, error) {
	for _, p := range f.plants {
		if p.HouseholdID == householdID {
			return p.TimeZone, nil
		}
	}
	return "", domain.ErrNotFound
}

func (f *fakeCareTasks) SetStatus(ctx context.Context, householdID, id uuid.UUID, status domain.TaskStatus, nextDueOn *civil.Date) (domain.CareTask, error) {
	t, err := f.Get(ctx, householdID, id)
	if err != nil {
		return domain.CareTask{}, err
	}
	t.Status = status
	if nextDueOn != nil {
		t.NextDueOn = *nextDueOn
	}
	f.byID[id] = t
	f.statuses = append(f.statuses, status)
	return t, nil
}

// taskFixture は、householdFixture の世帯とメンバーに、植物（マスタの水やりの目安つき）と、その水やりタスク、「その他」のタスクを加えた状態を用意する。
type taskFixture struct {
	householdFixture
	plant, watering, other uuid.UUID
	tasks                  *fakeCareTasks
	svc                    *app.CareTaskService
}

// taskNow は、東京で 2026-11-04 の朝（UTC では 11/3）である。今日の日付は世帯のタイムゾーンで決まることを確かめる。
var taskNow = time.Date(2026, 11, 3, 20, 0, 0, 0, time.UTC)

func newTaskFixture() taskFixture {
	hf := newHouseholdFixture()
	f := taskFixture{householdFixture: hf, plant: uuid.Must(uuid.NewV7()), watering: uuid.Must(uuid.NewV7()), other: uuid.Must(uuid.NewV7())}
	f.tasks = &fakeCareTasks{
		plants: map[uuid.UUID]app.TaskPlant{f.plant: {
			HouseholdID: hf.household, TimeZone: "Asia/Tokyo",
			Care: &domain.SpeciesCare{Watering: &domain.WateringAdvice{Condition: "鉢土の表面が乾いてきたら"}},
		}},
		byID: map[uuid.UUID]domain.CareTask{},
	}
	f.tasks.byID[f.watering] = domain.CareTask{ID: f.watering, HouseholdID: hf.household, PlantID: f.plant, CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareWatering}}
	f.tasks.byID[f.other] = domain.CareTask{ID: f.other, HouseholdID: hf.household, PlantID: f.plant, CareTaskProfile: domain.CareTaskProfile{Kind: domain.CareOther, CustomName: "葉水"}}
	f.svc = app.NewCareTaskService(f.tasks, hf.authz, func() time.Time { return taskNow })
	return f
}

var weekly = &domain.IntervalInput{Value: 1, Unit: "week"}

func TestCareTaskService_List(t *testing.T) {
	t.Parallel()

	t.Run("閲覧者もタスクを見られる", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		got, err := f.svc.List(t.Context(), f.viewer, f.plant)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Errorf("tasks = %+v", got)
		}
	})

	t.Run("ほかの世帯の植物・存在しない植物は 404", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		for _, tc := range []struct{ user, plant uuid.UUID }{{f.stranger, f.plant}, {f.owner, uuid.Must(uuid.NewV7())}} {
			if _, err := f.svc.List(t.Context(), tc.user, tc.plant); !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("err = %v, want ErrNotFound", err)
			}
		}
	})
}

func TestCareTaskService_Defaults(t *testing.T) {
	t.Parallel()

	t.Run("植物マスタの水やりの実行条件を、初期値として返す", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		got, err := f.svc.Defaults(t.Context(), f.viewer, f.plant)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0] != (domain.TaskDefault{Kind: domain.CareWatering, ConditionText: "鉢土の表面が乾いてきたら"}) {
			t.Errorf("defaults = %+v", got)
		}
	})

	t.Run("ほかの世帯の植物は 404", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		if _, err := f.svc.Defaults(t.Context(), f.stranger, f.plant); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestCareTaskService_Create(t *testing.T) {
	t.Parallel()

	t.Run("編集者は作成でき、初回の予定日を省くと、世帯のタイムゾーンでの今日にする", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		got, err := f.svc.Create(t.Context(), f.editor, f.plant, domain.CareTaskInput{Kind: ptr("fertilizing"), Interval: weekly})
		if err != nil {
			t.Fatal(err)
		}
		if got.HouseholdID != f.household || got.PlantID != f.plant || got.Kind != domain.CareFertilizing {
			t.Errorf("task = %+v", got)
		}
		if want := (civil.Date{Year: 2026, Month: 11, Day: 4}); got.NextDueOn != want {
			t.Errorf("next due = %v, want %v (today in Tokyo)", got.NextDueOn, want)
		}
	})

	t.Run("入力の誤りは 422 にあたるエラーで、保存しない", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		_, err := f.svc.Create(t.Context(), f.editor, f.plant, domain.CareTaskInput{Kind: ptr("other"), Interval: weekly})
		assertFields(t, err, map[string]string{"customName": "required"})
		if len(f.tasks.created) != 0 {
			t.Errorf("created = %+v", f.tasks.created)
		}
	})

	t.Run("閲覧者は作成できない", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		if _, err := f.svc.Create(t.Context(), f.viewer, f.plant, domain.CareTaskInput{Kind: ptr("watering"), Interval: weekly}); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("メンバーでない人には、入力の誤りより先に 404 を返す", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		if _, err := f.svc.Create(t.Context(), f.stranger, f.plant, domain.CareTaskInput{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

// taskCase は、タスクの ID だけで呼ぶ操作（変更・削除）の、権限の確かめ方の共通のケースである。
type taskCase struct {
	name    string
	user    func(taskFixture) uuid.UUID
	id      func(taskFixture) uuid.UUID
	wantErr error
}

var taskWriteDenied = []taskCase{
	{name: "閲覧者はできない", user: func(f taskFixture) uuid.UUID { return f.viewer }, id: func(f taskFixture) uuid.UUID { return f.watering }, wantErr: domain.ErrForbidden},
	{name: "ほかの世帯のタスクは 404", user: func(f taskFixture) uuid.UUID { return f.stranger }, id: func(f taskFixture) uuid.UUID { return f.watering }, wantErr: domain.ErrNotFound},
	{name: "存在しないタスクは 404", user: func(f taskFixture) uuid.UUID { return f.owner }, id: func(taskFixture) uuid.UUID { return uuid.Must(uuid.NewV7()) }, wantErr: domain.ErrNotFound},
}

func TestCareTaskService_Update(t *testing.T) {
	t.Parallel()

	t.Run("編集者は周期と予定日を変更できる", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		if _, err := f.svc.Update(t.Context(), f.editor, f.watering, domain.CareTaskInput{Interval: weekly, NextDueOn: ptr("2026-11-10")}); err != nil {
			t.Fatal(err)
		}
		p := f.tasks.patched
		if len(p) != 1 || p[0].Interval == nil || p[0].NextDueOn == nil || *p[0].NextDueOn != (civil.Date{Year: 2026, Month: 11, Day: 10}) {
			t.Errorf("patched = %+v", p)
		}
	})

	t.Run("作業名は、変更するタスクの種類で確かめる", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		_, err := f.svc.Update(t.Context(), f.editor, f.other, domain.CareTaskInput{CustomName: ptr("")})
		assertFields(t, err, map[string]string{"customName": "required"})
		if _, err := f.svc.Update(t.Context(), f.editor, f.watering, domain.CareTaskInput{CustomName: ptr("")}); err != nil {
			t.Errorf("watering: err = %v (custom name is ignored)", err)
		}
	})

	for _, tt := range taskWriteDenied {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newTaskFixture()
			if _, err := f.svc.Update(t.Context(), tt.user(f), tt.id(f), domain.CareTaskInput{Interval: weekly}); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(f.tasks.patched) != 0 {
				t.Errorf("patched = %+v, want none", f.tasks.patched)
			}
		})
	}
}

func TestCareTaskService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("編集者は削除できる", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		if err := f.svc.Delete(t.Context(), f.editor, f.watering); err != nil {
			t.Fatal(err)
		}
		if len(f.tasks.deleted) != 1 {
			t.Errorf("deleted = %v", f.tasks.deleted)
		}
	})

	for _, tt := range taskWriteDenied {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newTaskFixture()
			if err := f.svc.Delete(t.Context(), tt.user(f), tt.id(f)); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(f.tasks.deleted) != 0 {
				t.Errorf("deleted = %v, want none", f.tasks.deleted)
			}
		})
	}
}

func TestCareTaskService_Pause(t *testing.T) {
	t.Parallel()

	t.Run("編集者は一時停止でき、次回予定はそのまま残る", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		due := civil.Date{Year: 2026, Month: 11, Day: 10}
		f.setTask(f.watering, domain.TaskActive, due)
		got, err := f.svc.Pause(t.Context(), f.editor, f.watering)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != domain.TaskPaused || got.NextDueOn != due {
			t.Errorf("got %+v", got)
		}
	})

	for _, tt := range taskWriteDenied {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newTaskFixture()
			if _, err := f.svc.Pause(t.Context(), tt.user(f), tt.id(f)); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(f.tasks.statuses) != 0 {
				t.Errorf("statuses = %v, want none", f.tasks.statuses)
			}
		})
	}
}

func TestCareTaskService_Resume(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		due  civil.Date
		want civil.Date
	}{
		// taskNow は UTC では 11/3 だが、世帯（東京）の今日の 11/4 にする
		{"次回予定が過ぎていたら、世帯のタイムゾーンでの今日にする", civil.Date{Year: 2026, Month: 10, Day: 1}, civil.Date{Year: 2026, Month: 11, Day: 4}},
		{"次回予定がまだ先なら、そのまま使う", civil.Date{Year: 2026, Month: 11, Day: 20}, civil.Date{Year: 2026, Month: 11, Day: 20}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newTaskFixture()
			f.setTask(f.watering, domain.TaskPaused, tt.due)
			got, err := f.svc.Resume(t.Context(), f.editor, f.watering)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != domain.TaskActive || got.NextDueOn != tt.want {
				t.Errorf("got %+v, want next due %v", got, tt.want)
			}
		})
	}

	t.Run("有効なタスクを再開しても、期限切れの予定日は動かさない", func(t *testing.T) {
		t.Parallel()
		f := newTaskFixture()
		due := civil.Date{Year: 2026, Month: 10, Day: 1}
		f.setTask(f.watering, domain.TaskActive, due)
		got, err := f.svc.Resume(t.Context(), f.editor, f.watering)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != domain.TaskActive || got.NextDueOn != due || len(f.tasks.statuses) != 0 {
			t.Errorf("got %+v, statuses = %v", got, f.tasks.statuses)
		}
	})

	for _, tt := range taskWriteDenied {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newTaskFixture()
			f.setTask(f.watering, domain.TaskPaused, civil.Date{Year: 2026, Month: 10, Day: 1})
			if _, err := f.svc.Resume(t.Context(), tt.user(f), tt.id(f)); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(f.tasks.statuses) != 0 {
				t.Errorf("statuses = %v, want none", f.tasks.statuses)
			}
		})
	}
}

func (f taskFixture) setTask(id uuid.UUID, status domain.TaskStatus, due civil.Date) {
	t := f.tasks.byID[id]
	t.Status, t.NextDueOn = status, due
	f.tasks.byID[id] = t
}
