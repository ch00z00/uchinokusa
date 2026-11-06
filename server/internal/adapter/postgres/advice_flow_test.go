package postgres_test

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/postgres"
	"github.com/ch00z00/uchinokusa/server/internal/advisor"
	"github.com/ch00z00/uchinokusa/server/internal/app"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// adviceFlow は、植え替えの提案の流れ（植物の登録 → 提案 → 採用・見送り → 予定日の変更）を、
// 本物のリポジトリ・サービス・プロバイダをつないで確かめるための状態である（タスク 10.2）。
type adviceFlow struct {
	suggestionFixture
	editor      uuid.UUID
	plants      *app.PlantService
	tasks       *app.CareTaskService
	suggestions *app.SuggestionService
}

// adviceNow は、日本時間の 2026-11-06 9:00 である。
var adviceNow = time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)

func newAdviceFlow(t *testing.T) adviceFlow {
	t.Helper()
	f := adviceFlow{suggestionFixture: newSuggestionFixture(t)}
	f.editor = f.sakura
	addMember(t, f.pool, f.home, f.editor, domain.RoleEditor)
	authz := app.NewAuthorizer(postgres.NewMembershipRepository(f.pool))
	registry, err := advisor.NewRegistry(advisor.Repotting{})
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return adviceNow }
	f.suggestions = app.NewSuggestionService(f.suggestionFixture.suggestions, registry, authz, now, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	f.plants = app.NewPlantService(postgres.NewPlantRepository(f.pool), authz, f.suggestions)
	f.tasks = app.NewCareTaskService(postgres.NewCareTaskRepository(f.pool), authz, now, f.suggestions)
	return f
}

// active は、今日の時点で有効な提案を返す。
func (f adviceFlow) active() []domain.Suggestion {
	f.t.Helper()
	list, err := f.suggestions.List(f.ctx, f.editor, f.home, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	return list
}

// warnings は、タスクへの、やらない方がよいことの助言の予定日（MessageParams の date）を返す。
func (f adviceFlow) warnings(task uuid.UUID) []string {
	f.t.Helper()
	var out []string
	for _, s := range f.active() {
		if s.Kind == domain.SuggestionAvoid && s.TaskID == task {
			out = append(out, s.ValidUntil.String())
		}
	}
	return out
}

func (f adviceFlow) setDue(task uuid.UUID, due string) {
	f.t.Helper()
	if _, err := f.tasks.Update(f.ctx, f.editor, task, domain.CareTaskInput{NextDueOn: &due}); err != nil {
		f.t.Fatal(err)
	}
}

func TestAdviceFlow_RepottingFromPlantRegistration(t *testing.T) {
	t.Parallel()
	f := newAdviceFlow(t)

	// 購入日つきで植物を登録すると、植え替えタスクの提案ができる（要件17-1・17-2）
	plant, err := f.plants.Create(f.ctx, f.editor, f.home, domain.PlantInput{Name: ptrTo("パキラ"), AcquiredOn: ptrTo("2026-11-01")})
	if err != nil {
		t.Fatal(err)
	}
	list := f.active()
	if len(list) != 1 {
		t.Fatalf("suggestions = %+v", list)
	}
	initial := list[0]
	if initial.ProviderID != "repotting.v1" || initial.PlantID != plant.ID || initial.Action != domain.SuggestionCreateTask ||
		initial.ProposedDate != (civil.Date{Year: 2027, Month: 5, Day: 1}) || initial.TaskSchedule.Base != advisor.RepottingInterval {
		t.Fatalf("initial = %+v", initial)
	}

	// 提案日を真夏に変えて採用すると、その日を予定日にした植え替えタスクを作り（要件17-5）、
	// やらない方がよいことの助言を出す（要件17-8）
	_, task, err := f.suggestions.Accept(f.ctx, f.editor, initial.ID, domain.AcceptInput{Date: ptrTo("2027-07-20")})
	if err != nil {
		t.Fatal(err)
	}
	if task.Kind != domain.CareRepotting || task.NextDueOn != (civil.Date{Year: 2027, Month: 7, Day: 20}) {
		t.Fatalf("task = %+v", task)
	}
	if got := f.warnings(task.ID); len(got) != 1 || got[0] != "2027-07-20" {
		t.Errorf("warnings after accept = %v", got)
	}

	// 適期に変えると、前の予定日についての助言は消える
	f.setDue(task.ID, "2027-09-10")
	if got := f.warnings(task.ID); len(got) != 0 {
		t.Errorf("warnings after moving to suitable month = %v", got)
	}
	// 真冬に変えると、その日の助言を出す
	f.setDue(task.ID, "2027-01-15")
	if got := f.warnings(task.ID); len(got) != 1 || got[0] != "2027-01-15" {
		t.Errorf("warnings after moving to midwinter = %v", got)
	}
	// 一度消えた日に戻すと、助言を出し直す
	f.setDue(task.ID, "2027-07-20")
	if got := f.warnings(task.ID); len(got) != 1 || got[0] != "2027-07-20" {
		t.Errorf("warnings after moving back = %v", got)
	}

	// 見送った助言は、同じ日に戻しても出し直さない（要件17-6）
	warning := f.active()[0]
	if _, err := f.suggestions.Dismiss(f.ctx, f.editor, warning.ID); err != nil {
		t.Fatal(err)
	}
	f.setDue(task.ID, "2027-09-10")
	f.setDue(task.ID, "2027-07-20")
	if got := f.warnings(task.ID); len(got) != 0 {
		t.Errorf("warnings after dismissing = %v", got)
	}

	// タスクを消すと、助言も消える
	f.setDue(task.ID, "2027-08-05")
	if got := f.warnings(task.ID); len(got) != 1 {
		t.Fatalf("warnings before delete = %v", got)
	}
	if err := f.tasks.Delete(f.ctx, f.editor, task.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.warnings(task.ID); len(got) != 0 {
		t.Errorf("warnings after delete = %v", got)
	}
}

func TestAdviceFlow_DismissedRepottingIsNotRepeated(t *testing.T) {
	t.Parallel()
	f := newAdviceFlow(t)
	plant, err := f.plants.Create(f.ctx, f.editor, f.home, domain.PlantInput{Name: ptrTo("ポトス"), AcquiredOn: ptrTo("2026-10-01")})
	if err != nil {
		t.Fatal(err)
	}
	initial := f.active()[0]

	// 見送ると、植え替えタスクは作らず（要件17-6）、同じ植物にもう一度評価しても、同じ提案は出さない
	if _, err := f.suggestions.Dismiss(f.ctx, f.editor, initial.ID); err != nil {
		t.Fatal(err)
	}
	f.suggestions.Evaluate(f.ctx, app.AdviceEvent{Trigger: advisor.TriggerPlantCreated, PlantID: plant.ID})
	if list := f.active(); len(list) != 0 {
		t.Errorf("suggestions = %+v", list)
	}
	if n := f.count(`SELECT count(*) FROM care_tasks WHERE plant_id = $1`, plant.ID); n != 0 {
		t.Errorf("tasks = %d", n)
	}
}

func TestAdviceFlow_NoSuggestionWithoutAcquiredDate(t *testing.T) {
	t.Parallel()
	f := newAdviceFlow(t)
	if _, err := f.plants.Create(f.ctx, f.editor, f.home, domain.PlantInput{Name: ptrTo("サンスベリア")}); err != nil {
		t.Fatal(err)
	}
	if list := f.active(); len(list) != 0 {
		t.Errorf("suggestions = %+v", list)
	}
}
