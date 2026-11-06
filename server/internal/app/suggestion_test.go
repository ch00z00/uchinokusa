package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/advisor"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// fakeSuggestions は、app.SuggestionRepository の偽物である。提案をメモリに持ち、重複防止のキーで重複を防ぐ。
type fakeSuggestions struct {
	subject    advisor.Subject
	byID       map[uuid.UUID]domain.Suggestion
	listedOn   []civil.Date
	acceptedOn []civil.Date
	retracted  []retraction
}

type retraction struct {
	providerID string
	taskID     uuid.UUID
	keep       []string
}

func (f *fakeSuggestions) Subject(_ context.Context, plantID uuid.UUID) (advisor.Subject, error) {
	if plantID != f.subject.Plant.ID {
		return advisor.Subject{}, domain.ErrNotFound
	}
	return f.subject, nil
}

func (f *fakeSuggestions) Save(_ context.Context, s domain.Suggestion) (domain.Suggestion, bool, error) {
	for _, existing := range f.byID {
		if existing.HouseholdID == s.HouseholdID && existing.DedupeKey == s.DedupeKey {
			return domain.Suggestion{}, false, nil
		}
	}
	s.ID = uuid.Must(uuid.NewV7())
	f.byID[s.ID] = s
	return s, true, nil
}

func (f *fakeSuggestions) Retract(_ context.Context, householdID uuid.UUID, providerID string, taskID uuid.UUID, keep []string) error {
	f.retracted = append(f.retracted, retraction{providerID: providerID, taskID: taskID, keep: keep})
	return nil
}

func (f *fakeSuggestions) HouseholdOf(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	s, ok := f.byID[id]
	if !ok {
		return uuid.Nil, domain.ErrNotFound
	}
	return s.HouseholdID, nil
}

func (f *fakeSuggestions) TimeZoneOf(context.Context, uuid.UUID) (string, error) {
	return "Asia/Tokyo", nil
}

func (f *fakeSuggestions) ListActive(_ context.Context, householdID uuid.UUID, on civil.Date) ([]domain.Suggestion, error) {
	f.listedOn = append(f.listedOn, on)
	return []domain.Suggestion{}, nil
}

func (f *fakeSuggestions) Accept(_ context.Context, householdID, id, actorID uuid.UUID, in domain.AcceptInput, today civil.Date) (domain.Suggestion, domain.CareTask, error) {
	f.acceptedOn = append(f.acceptedOn, today)
	s := f.byID[id]
	plan, err := domain.PlanAcceptance(s, in, today)
	if err != nil {
		return domain.Suggestion{}, domain.CareTask{}, err
	}
	s.Status, s.RespondedBy = domain.SuggestionAccepted, uuid.NullUUID{UUID: actorID, Valid: true}
	f.byID[id] = s
	var task domain.CareTask
	if plan.Task != nil {
		task = domain.CareTask{ID: uuid.Must(uuid.NewV7()), CareTaskProfile: plan.Task.CareTaskProfile, NextDueOn: plan.Task.NextDueOn}
	}
	return s, task, nil
}

func (f *fakeSuggestions) Dismiss(_ context.Context, householdID, id, actorID uuid.UUID) (domain.Suggestion, error) {
	s := f.byID[id]
	if err := domain.CheckDismiss(s); err != nil {
		return domain.Suggestion{}, err
	}
	s.Status, s.RespondedBy = domain.SuggestionDismissed, uuid.NullUUID{UUID: actorID, Valid: true}
	f.byID[id] = s
	return s, nil
}

// fakeAdvisor は、決まった提案（またはエラー・panic）を返すプロバイダである。受け取った情報を記録する。
type fakeAdvisor struct {
	id        string
	triggers  []advisor.Trigger
	proposals []advisor.Proposal
	err       error
	panics    bool
	got       []advisor.Context
}

func (a *fakeAdvisor) ID() string                  { return a.id }
func (a *fakeAdvisor) Triggers() []advisor.Trigger { return a.triggers }
func (a *fakeAdvisor) Advise(_ context.Context, in advisor.Context) ([]advisor.Proposal, error) {
	a.got = append(a.got, in)
	if a.panics {
		panic("boom")
	}
	return a.proposals, a.err
}

func proposal(key string) advisor.Proposal {
	return advisor.Proposal{
		Kind: domain.SuggestionDo, Action: domain.SuggestionCreateTask, TaskKind: domain.CareRepotting,
		TaskSchedule: domain.Schedule{Base: domain.Interval{Value: 12, Unit: domain.IntervalMonth}},
		ProposedDate: civil.Date{Year: 2027, Month: 5, Day: 1}, MessageKey: "m", ReasonKey: "r",
		ValidFrom: civil.Date{Year: 2026, Month: 11, Day: 6}, DedupeKey: key,
	}
}

// suggestionNow は、日本時間の 2026-11-06 0:30（UTC では 11/5）である。
var suggestionNow = time.Date(2026, 11, 5, 15, 30, 0, 0, time.UTC)

type suggestionFixture struct {
	householdFixture
	plant uuid.UUID
	repo  *fakeSuggestions
	logs  *bytes.Buffer
}

func newSuggestionFixture() suggestionFixture {
	hf := newHouseholdFixture()
	f := suggestionFixture{householdFixture: hf, plant: uuid.Must(uuid.NewV7()), logs: &bytes.Buffer{}}
	f.repo = &fakeSuggestions{byID: map[uuid.UUID]domain.Suggestion{}, subject: advisor.Subject{
		Household: domain.Household{ID: hf.household, TimeZone: "Asia/Tokyo"},
		Plant:     domain.Plant{ID: f.plant, HouseholdID: hf.household},
	}}
	return f
}

func (f suggestionFixture) service(advisors ...advisor.Advisor) *app.SuggestionService {
	r, err := advisor.NewRegistry(advisors...)
	if err != nil {
		panic(err)
	}
	return app.NewSuggestionService(f.repo, r, f.authz, func() time.Time { return suggestionNow }, slog.New(slog.NewTextHandler(f.logs, nil)))
}

// addSuggestion は、未回答の提案を1件入れる。
func (f suggestionFixture) addSuggestion(action domain.SuggestionAction) uuid.UUID {
	s := domain.Suggestion{
		ID: uuid.Must(uuid.NewV7()), HouseholdID: f.household, PlantID: f.plant, Kind: domain.SuggestionDo, Action: action,
		TaskKind: domain.CareRepotting, TaskSchedule: domain.Schedule{Base: domain.Interval{Value: 12, Unit: domain.IntervalMonth}},
		ProposedDate: civil.Date{Year: 2027, Month: 5, Day: 1}, ValidFrom: civil.Date{Year: 2026, Month: 11, Day: 1}, Status: domain.SuggestionPending,
	}
	f.repo.byID[s.ID] = s
	return s.ID
}

func TestSuggestionService_Evaluate(t *testing.T) {
	t.Parallel()

	t.Run("きっかけのプロバイダを呼び、返した提案を提案元つきで保存する", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		created := &fakeAdvisor{id: "repotting.v1", triggers: []advisor.Trigger{advisor.TriggerPlantCreated}, proposals: []advisor.Proposal{proposal("k1")}}
		daily := &fakeAdvisor{id: "weather.v1", triggers: []advisor.Trigger{advisor.TriggerDaily}, proposals: []advisor.Proposal{proposal("k2")}}
		got := f.service(created, daily).Evaluate(t.Context(), app.AdviceEvent{Trigger: advisor.TriggerPlantCreated, PlantID: f.plant})

		if len(got) != 1 || got[0].ProviderID != "repotting.v1" || got[0].HouseholdID != f.household || got[0].PlantID != f.plant || got[0].DedupeKey != "k1" {
			t.Fatalf("suggestions = %+v", got)
		}
		if len(daily.got) != 0 {
			t.Errorf("daily advisor was called")
		}
		// 今日は、世帯のタイムゾーンで決める
		if in := created.got[0]; in.Trigger != advisor.TriggerPlantCreated || in.Today != (civil.Date{Year: 2026, Month: 11, Day: 6}) || in.Plant.ID != f.plant {
			t.Errorf("context = %+v", in)
		}
	})

	t.Run("同じ提案は二度作らない", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		svc := f.service(&fakeAdvisor{id: "repotting.v1", triggers: []advisor.Trigger{advisor.TriggerPlantCreated}, proposals: []advisor.Proposal{proposal("k1")}})
		ev := app.AdviceEvent{Trigger: advisor.TriggerPlantCreated, PlantID: f.plant}
		svc.Evaluate(t.Context(), ev)
		if got := svc.Evaluate(t.Context(), ev); len(got) != 0 || len(f.repo.byID) != 1 {
			t.Errorf("second = %+v, stored = %d", got, len(f.repo.byID))
		}
	})

	t.Run("失敗したプロバイダは記録して飛ばし、ほかのプロバイダの提案は保存する", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		trig := []advisor.Trigger{advisor.TriggerPlantCreated}
		// エラーを返したプロバイダの提案は、一緒に返されても保存しない（途中までの結果かもしれないため）
		failing := &fakeAdvisor{id: "fail.v1", triggers: trig, proposals: []advisor.Proposal{proposal("partial")}, err: errors.New("species lookup failed")}
		panicking := &fakeAdvisor{id: "panic.v1", triggers: trig, panics: true}
		broken := proposal("broken")
		broken.MessageKey = ""
		invalid := &fakeAdvisor{id: "invalid.v1", triggers: trig, proposals: []advisor.Proposal{broken, proposal("ok2")}}
		ok := &fakeAdvisor{id: "ok.v1", triggers: trig, proposals: []advisor.Proposal{proposal("ok1")}}

		got := f.service(failing, panicking, invalid, ok).Evaluate(t.Context(), app.AdviceEvent{Trigger: advisor.TriggerPlantCreated, PlantID: f.plant})
		var keys []string
		for _, s := range got {
			keys = append(keys, s.DedupeKey)
		}
		if !slices.Equal(keys, []string{"ok2", "ok1"}) {
			t.Errorf("keys = %v", keys)
		}
		for _, id := range []string{"fail.v1", "panic.v1", "invalid.v1"} {
			if !strings.Contains(f.logs.String(), id) {
				t.Errorf("log does not mention %s:\n%s", id, f.logs.String())
			}
		}
	})

	t.Run("アーカイブした植物には、プロバイダを呼ばない", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		archivedAt := suggestionNow
		f.repo.subject.Plant.ArchivedAt = &archivedAt
		a := &fakeAdvisor{id: "repotting.v1", triggers: []advisor.Trigger{advisor.TriggerPlantCreated}, proposals: []advisor.Proposal{proposal("k1")}}
		if got := f.service(a).Evaluate(t.Context(), app.AdviceEvent{Trigger: advisor.TriggerPlantCreated, PlantID: f.plant}); len(got) != 0 || len(a.got) != 0 {
			t.Errorf("suggestions = %+v, calls = %d", got, len(a.got))
		}
	})

	t.Run("植物がなくても、エラーにせず記録だけする", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		got := f.service().Evaluate(t.Context(), app.AdviceEvent{Trigger: advisor.TriggerPlantCreated, PlantID: uuid.Must(uuid.NewV7())})
		if len(got) != 0 || f.logs.Len() == 0 {
			t.Errorf("suggestions = %+v, logs = %q", got, f.logs.String())
		}
	})

	t.Run("予定日の変更では、プロバイダが今回返さなかった、そのタスクへの前の提案を撤回する", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		task := uuid.Must(uuid.NewV7())
		trig := []advisor.Trigger{advisor.TriggerTaskDueChanged}
		ok := &fakeAdvisor{id: "repotting.v1", triggers: trig, proposals: []advisor.Proposal{proposal("k1")}}
		failing := &fakeAdvisor{id: "fail.v1", triggers: trig, err: errors.New("boom")}
		f.service(ok, failing).Evaluate(t.Context(), app.AdviceEvent{Trigger: advisor.TriggerTaskDueChanged, PlantID: f.plant, TaskID: task})
		// 失敗したプロバイダの前の提案は、今の考えがわからないので撤回しない
		want := []retraction{{providerID: "repotting.v1", taskID: task, keep: []string{"k1"}}}
		if !reflect.DeepEqual(f.repo.retracted, want) {
			t.Errorf("retracted = %+v, want %+v", f.repo.retracted, want)
		}
	})

	t.Run("植物への評価では、撤回しない", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		a := &fakeAdvisor{id: "repotting.v1", triggers: []advisor.Trigger{advisor.TriggerPlantCreated}}
		f.service(a).Evaluate(t.Context(), app.AdviceEvent{Trigger: advisor.TriggerPlantCreated, PlantID: f.plant})
		if len(f.repo.retracted) != 0 {
			t.Errorf("retracted = %+v", f.repo.retracted)
		}
	})

	t.Run("予定日の変更では、対象のタスクを渡す", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		a := &fakeAdvisor{id: "repotting.v1", triggers: []advisor.Trigger{advisor.TriggerTaskDueChanged}}
		task := uuid.Must(uuid.NewV7())
		f.service(a).Evaluate(t.Context(), app.AdviceEvent{Trigger: advisor.TriggerTaskDueChanged, PlantID: f.plant, TaskID: task})
		if len(a.got) != 1 || a.got[0].TaskID != task {
			t.Errorf("context = %+v", a.got)
		}
	})
}

func TestSuggestionService_List(t *testing.T) {
	t.Parallel()

	t.Run("閲覧者も見られ、日付を省くと世帯のタイムゾーンでの今日にする", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		if _, err := f.service().List(t.Context(), f.viewer, f.household, nil); err != nil {
			t.Fatal(err)
		}
		if f.repo.listedOn[0] != (civil.Date{Year: 2026, Month: 11, Day: 6}) {
			t.Errorf("on = %v", f.repo.listedOn)
		}
	})

	t.Run("アプリが送った日付で絞る", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		if _, err := f.service().List(t.Context(), f.viewer, f.household, ptr("2026-11-10")); err != nil {
			t.Fatal(err)
		}
		if f.repo.listedOn[0] != (civil.Date{Year: 2026, Month: 11, Day: 10}) {
			t.Errorf("on = %v", f.repo.listedOn)
		}
	})

	t.Run("メンバーでない人には、入力の誤りより先に 404 を返す", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		if _, err := f.service().List(t.Context(), f.stranger, f.household, ptr("x")); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestSuggestionService_Accept(t *testing.T) {
	t.Parallel()

	t.Run("編集者が採用すると、提案日を予定日にしたタスクを作り、対応した人を記録する", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		id := f.addSuggestion(domain.SuggestionCreateTask)
		s, task, err := f.service().Accept(t.Context(), f.editor, id, domain.AcceptInput{})
		if err != nil {
			t.Fatal(err)
		}
		if s.Status != domain.SuggestionAccepted || s.RespondedBy.UUID != f.editor || task.Kind != domain.CareRepotting || task.NextDueOn != (civil.Date{Year: 2027, Month: 5, Day: 1}) {
			t.Errorf("suggestion = %+v, task = %+v", s, task)
		}
		if f.repo.acceptedOn[0] != (civil.Date{Year: 2026, Month: 11, Day: 6}) {
			t.Errorf("today = %v", f.repo.acceptedOn)
		}
	})

	t.Run("採用でタスクを作ったら、そのタスクで予定日の提案を評価する", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		id := f.addSuggestion(domain.SuggestionCreateTask)
		a := &fakeAdvisor{id: "repotting.v1", triggers: []advisor.Trigger{advisor.TriggerTaskDueChanged}}
		_, task, err := f.service(a).Accept(t.Context(), f.editor, id, domain.AcceptInput{Date: ptr("2027-07-20")})
		if err != nil {
			t.Fatal(err)
		}
		if len(a.got) != 1 || a.got[0].TaskID != task.ID || a.got[0].Plant.ID != f.plant {
			t.Errorf("contexts = %+v", a.got)
		}
	})

	t.Run("タスクを作らない提案の採用では、評価しない", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		id := f.addSuggestion(domain.SuggestionInfo)
		a := &fakeAdvisor{id: "repotting.v1", triggers: []advisor.Trigger{advisor.TriggerTaskDueChanged}}
		if _, _, err := f.service(a).Accept(t.Context(), f.editor, id, domain.AcceptInput{}); err != nil {
			t.Fatal(err)
		}
		if len(a.got) != 0 {
			t.Errorf("contexts = %+v", a.got)
		}
	})

	t.Run("閲覧者は採用できない", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		id := f.addSuggestion(domain.SuggestionCreateTask)
		if _, _, err := f.service().Accept(t.Context(), f.viewer, id, domain.AcceptInput{}); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("提案がなければ 404", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		if _, _, err := f.service().Accept(t.Context(), f.owner, uuid.Must(uuid.NewV7()), domain.AcceptInput{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestSuggestionService_Dismiss(t *testing.T) {
	t.Parallel()

	t.Run("見送ると、対応した人を記録する", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		id := f.addSuggestion(domain.SuggestionCreateTask)
		s, err := f.service().Dismiss(t.Context(), f.editor, id)
		if err != nil || s.Status != domain.SuggestionDismissed || s.RespondedBy.UUID != f.editor {
			t.Errorf("suggestion = %+v, %v", s, err)
		}
	})

	t.Run("閲覧者は見送れない", func(t *testing.T) {
		t.Parallel()
		f := newSuggestionFixture()
		id := f.addSuggestion(domain.SuggestionCreateTask)
		if _, err := f.service().Dismiss(t.Context(), f.viewer, id); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})
}
